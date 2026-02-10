/**
 * ========= CONFIG =========
 */
const SHEET_REIMBURSEMENT = "REIMBURSEMENT_REQUEST";
const SHEET_CASH_TX = "CASH_TRANSACTION";
const SHEET_CASH_ACCOUNT = "MASTER_CASH_ACCOUNT";

const STATUS_DRAFT = "Draft";
const STATUS_READY = "Ready";
const STATUS_POSTED = "Posted";

const REIMBURSE_PAYABLE_ACCOUNT = "2101"; // Reimbursement Payable (Liability)
const REIMBURSE_PAYABLE_DISPLAY = "2101 - Reimbursement Payable";

/**
 * ========= MENU =========
 */
function onOpen() {
  SpreadsheetApp.getUi()
    .createMenu("Accounting")
    .addItem("Post Reimbursement Batch", "openPostBatchDialog")
    .addToUi();
}

/**
 * ========= UI =========
 */
function openPostBatchDialog() {
  const html = HtmlService.createHtmlOutputFromFile("PostBatchDialog")
    .setWidth(420)
    .setHeight(380);

  SpreadsheetApp.getUi().showModalDialog(html, "Post Reimbursement Batch");
}

/**
 * ========= DATA FOR DIALOG =========
 */
function getActiveCashAccounts() {
  const ss = SpreadsheetApp.getActive();
  const sheet = ss.getSheetByName(SHEET_CASH_ACCOUNT);
  const values = sheet.getDataRange().getValues();

  const header = values[0];
  const idx = Object.fromEntries(header.map((h, i) => [h, i]));

  return values
    .slice(1)
    .filter((r) => r[idx.is_active] === true)
    .map((r) => ({
      code: r[idx.cash_account_code],
      display: r[idx.cash_account_display],
    }));
}

/**
 * ========= MAIN POSTING =========
 */
function postReimbursementBatch(form) {
  const ss = SpreadsheetApp.getActive();
  const reqSheet = ss.getSheetByName(SHEET_REIMBURSEMENT);
  const cashSheet = ss.getSheetByName(SHEET_CASH_TX);

  // 1. Parse and validate inputs
  const paymentDate = parseISODate(form.paymentDate);
  const cashAccountCode = form.cashAccountCode;
  const cashAccountDisplay = form.cashAccountDisplay;

  if (!paymentDate || isNaN(paymentDate.getTime())) {
    throw new Error("Invalid payment date");
  }
  if (!cashAccountCode) {
    throw new Error("Missing cash account");
  }

  // Validate cash account exists and is active
  const validCashAccounts = getActiveCashAccounts();
  const isValidCashAccount = validCashAccounts.some(
    (acc) => acc.code === cashAccountCode && acc.display === cashAccountDisplay
  );
  if (!isValidCashAccount) {
    throw new Error(`Invalid or inactive cash account: ${cashAccountCode}`);
  }

  console.log('=== Post Reimbursement Batch Started ===');
  console.log('Payment Date:', form.paymentDate);
  console.log('Cash Account:', cashAccountCode, '-', cashAccountDisplay);

  // 2. Get data
  const data = reqSheet.getDataRange().getValues();
  const header = data[0];
  const idx = Object.fromEntries(header.map((h, i) => [h, i]));

  // Validate required columns exist
  const requiredCols = ['status', 'posted_at', 'amount', 'expense_date', 'account_display', 'batch_id'];
  const missingCols = requiredCols.filter(col => idx[col] === undefined);
  if (missingCols.length > 0) {
    throw new Error(`Missing required columns in REIMBURSEMENT_REQUEST: ${missingCols.join(', ')}`);
  }

  console.log('Columns:', header.join(', '));

  const readyRows = data
    .slice(1)
    .map((r, i) => ({ row: r, rowIndex: i + 2 }))
    .filter((x) => x.row[idx.status] === STATUS_READY);

  console.log('Ready rows:', readyRows.length);

  if (readyRows.length === 0) {
    throw new Error("No rows with status = Ready");
  }

  // 3. VALIDATE ALL ROWS FIRST (before any writes)
  readyRows.forEach(({ row, rowIndex }) => {
    const amount = Number(row[idx.amount]);
    if (!amount || amount <= 0) {
      console.log('Validation failed at row', rowIndex, '- amount:', row[idx.amount]);
      throw new Error(`Invalid amount at row ${rowIndex}: got "${row[idx.amount]}"`);
    }
    if (paymentDate < row[idx.expense_date]) {
      console.log('Validation failed at row', rowIndex, '- expense_date:', row[idx.expense_date], 'payment_date:', paymentDate);
      throw new Error(`Payment date before expense date (row ${rowIndex})`);
    }
    if (!row[idx.account_display]) {
      console.log('Validation failed at row', rowIndex, '- account_display is empty');
      throw new Error(`Missing account_display at row ${rowIndex}`);
    }
  });

  // 3b. Enforce single batch_id per posting run
  const batchIds = [...new Set(readyRows.map(({ row }) => row[idx.batch_id]))];
  if (batchIds.length > 1) {
    throw new Error(
      `Multiple batch IDs found: ${batchIds.join(", ")}. Post one batch at a time.`,
    );
  }
  const batchId = batchIds[0];
  console.log('Batch ID:', batchId);

  // 3c. Idempotency check - ensure batch not already posted
  const existingBatchIds = getExistingBatchIds(cashSheet);
  if (existingBatchIds.has(batchId)) {
    throw new Error(`Batch ${batchId} already posted to CASH_TRANSACTION`);
  }

  // 4. Build all transaction rows
  const now = new Date();
  const newTxRows = [];

  let nextTxNum = getNextTransactionNumber(cashSheet);

  readyRows.forEach(({ row }) => {
    const amount = Number(row[idx.amount]);

    // A. Expense leg - credits Reimbursement Payable (creates liability)
    newTxRows.push(
      buildCashTxRow({
        txId: formatTransactionId(nextTxNum++),
        date: row[idx.expense_date],
        itemDisplay: row[idx.item_display] || "",
        description: `Reimbursement expense - ${row[idx.description]}`,
        quantity: row[idx.qty] || 1,
        unitPrice: row[idx.unit_price] || amount,
        accountDisplay: row[idx.account_display],
        cashAccountDisplay: "",  // No cash movement - liability only
        batchId,
      }),
    );

    // B. Payment leg - debits liability, credits cash (clears liability)
    newTxRows.push(
      buildCashTxRow({
        txId: formatTransactionId(nextTxNum++),
        date: paymentDate,
        itemDisplay: "",
        description: `Reimbursement payment - ${row[idx.batch_id]}`,
        quantity: 1,
        unitPrice: amount,
        accountDisplay: REIMBURSE_PAYABLE_DISPLAY,
        cashAccountDisplay: cashAccountDisplay,
        batchId,
      }),
    );
  });

  // 5. APPEND TRANSACTIONS and UPDATE STATUS with partial failure handling
  const appendStartRow = getLastDataRow(cashSheet, 1) + 1; // Column A = transaction_id
  console.log('Appending', newTxRows.length, 'transactions starting at row', appendStartRow);

  try {
    appendCashTransactions(cashSheet, newTxRows);

    // Force write and verify rows were actually written
    SpreadsheetApp.flush();
    const newLastRow = getLastDataRow(cashSheet, 1); // Column A = transaction_id
    const expectedLastRow = appendStartRow + newTxRows.length - 1;
    console.log('Expected last row:', expectedLastRow, 'Actual last row:', newLastRow);

    if (newLastRow < expectedLastRow) {
      throw new Error(`Append failed: expected row ${expectedLastRow}, got ${newLastRow}. Check data validation on CASH_TRANSACTION.`);
    }

    // Batch status updates in single call to reduce partial failure window
    // Build status and posted_at values for all rows
    const statusColIdx = idx.status + 1;
    const postedAtColIdx = idx.posted_at + 1;

    // Sort rows by rowIndex to ensure contiguous updates
    const sortedRows = [...readyRows].sort((a, b) => a.rowIndex - b.rowIndex);

    // Check if rows are contiguous for batch update
    const isContiguous = sortedRows.every((row, i) =>
      i === 0 || row.rowIndex === sortedRows[i - 1].rowIndex + 1
    );

    if (isContiguous && sortedRows.length > 0) {
      // Batch update - all rows in single setValues() call
      const startRow = sortedRows[0].rowIndex;
      const statusUpdates = sortedRows.map(() => [STATUS_POSTED]);
      const postedAtUpdates = sortedRows.map(() => [now]);

      reqSheet.getRange(startRow, statusColIdx, sortedRows.length, 1).setValues(statusUpdates);
      reqSheet.getRange(startRow, postedAtColIdx, sortedRows.length, 1).setValues(postedAtUpdates);
      console.log('Batch updated status for rows', startRow, 'to', startRow + sortedRows.length - 1);
    } else {
      // Non-contiguous rows - update individually (fallback)
      console.log('Non-contiguous rows detected, updating individually');
      readyRows.forEach(({ rowIndex }) => {
        reqSheet.getRange(rowIndex, statusColIdx).setValue(STATUS_POSTED);
        reqSheet.getRange(rowIndex, postedAtColIdx).setValue(now);
      });
    }
  } catch (e) {
    throw new Error(
      `Posting failed: ${e.message}. Check CASH_TRANSACTION for partial data.`,
    );
  }

  console.log('=== Completed ===');
}

/**
 * ========= HELPERS =========
 */
function getLastDataRow(sheet, columnIndex = 1) {
  const data = sheet.getRange(1, columnIndex, sheet.getMaxRows(), 1).getValues();
  for (let i = data.length - 1; i >= 0; i--) {
    if (data[i][0] !== "" && data[i][0] !== null) {
      return i + 1; // Convert to 1-based row number
    }
  }
  return 1; // Return header row if no data
}

function buildCashTxRow({
  txId,
  date,
  itemDisplay,
  description,
  quantity,
  unitPrice,
  accountDisplay,
  cashAccountDisplay,
  batchId,
}) {
  return [
    txId,                 // 1. transaction_id
    date,                 // 2. transaction_date
    itemDisplay,          // 3. item_display
    "",                   // 4. item_code (formula)
    description,          // 5. description
    quantity,             // 6. quantity
    unitPrice,            // 7. unit_price
    "",                   // 8. amount (formula)
    "",                   // 9. line_type (formula)
    accountDisplay,       // 10. account_display
    "",                   // 11. account_code (formula)
    cashAccountDisplay,   // 12. cash_account_display
    "",                   // 13. cash_account_code (formula)
    "",                   // 14. outlet (optional)
    "",                   // 15. month (formula)
    "",                   // 16. line_type_backup (optional)
    batchId || "",        // 17. reimbursement_batch_id
  ];
}

function appendCashTransactions(sheet, rows) {
  if (!rows.length) return;
  const startRow = getLastDataRow(sheet, 1) + 1; // Column A = transaction_id
  sheet.getRange(startRow, 1, rows.length, rows[0].length).setValues(rows);
  return startRow;
}

function getExistingBatchIds(cashSheet) {
  const data = cashSheet.getDataRange().getValues();
  const header = data[0];
  const batchIdColIdx = header.indexOf("reimbursement_batch_id");
  if (batchIdColIdx === -1) return new Set();
  return new Set(data.slice(1).map((r) => r[batchIdColIdx]).filter(Boolean));
}

function getNextTransactionNumber(cashSheet) {
  const data = cashSheet.getDataRange().getValues();
  const header = data[0];
  const txIdColIdx = header.indexOf("transaction_id");
  if (txIdColIdx === -1) return 1;

  let maxNum = 0;
  data.slice(1).forEach((row) => {
    const txId = row[txIdColIdx];
    if (typeof txId === "string" && txId.startsWith("PCS")) {
      const num = parseInt(txId.slice(3), 10);
      if (!isNaN(num) && num > maxNum) {
        maxNum = num;
      }
    }
  });

  return maxNum + 1;
}

function formatTransactionId(num) {
  return "PCS" + String(num).padStart(6, "0");
}

function parseISODate(dateStr) {
  if (!dateStr || typeof dateStr !== "string") {
    return null;
  }
  const parts = dateStr.split("-");
  if (parts.length !== 3) {
    return null;
  }
  const [y, m, d] = parts.map(Number);
  if (isNaN(y) || isNaN(m) || isNaN(d)) {
    return null;
  }
  return new Date(y, m - 1, d);
}
