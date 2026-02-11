-- Accounting Module: master data + transaction tables
-- See docs/plans/2026-02-11-accounting-module-design.md

-- Chart of Accounts (~32 rows)
CREATE TABLE acct_accounts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_code  VARCHAR(10) UNIQUE NOT NULL,
    account_name  VARCHAR(100) NOT NULL,
    account_type  VARCHAR(20) NOT NULL,
    line_type     VARCHAR(20) NOT NULL,
    is_active     BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE acct_accounts ADD CONSTRAINT chk_acct_accounts_type
  CHECK (account_type IN ('Asset', 'Liability', 'Equity', 'Revenue', 'Expense'));

ALTER TABLE acct_accounts ADD CONSTRAINT chk_acct_accounts_line_type
  CHECK (line_type IN ('ASSET', 'INVENTORY', 'EXPENSE', 'SALES', 'COGS', 'LIABILITY', 'CAPITAL', 'DRAWING'));

-- Inventory Items (~88 rows)
CREATE TABLE acct_items (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_code      VARCHAR(20) UNIQUE NOT NULL,
    item_name      VARCHAR(100) NOT NULL,
    item_category  VARCHAR(30) NOT NULL,
    unit           VARCHAR(10) NOT NULL,
    is_inventory   BOOLEAN NOT NULL DEFAULT true,
    is_active      BOOLEAN NOT NULL DEFAULT true,
    average_price  DECIMAL(12,2),
    last_price     DECIMAL(12,2),
    for_hpp        DECIMAL(12,2),
    keywords       TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE acct_items ADD CONSTRAINT chk_acct_items_category
  CHECK (item_category IN ('Raw Material', 'Packaging', 'Consumable'));

-- Cash/Bank Accounts (~7 rows)
CREATE TABLE acct_cash_accounts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cash_account_code   VARCHAR(20) UNIQUE NOT NULL,
    cash_account_name   VARCHAR(100) NOT NULL,
    bank_name           VARCHAR(50),
    ownership           VARCHAR(20) NOT NULL,
    is_active           BOOLEAN NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE acct_cash_accounts ADD CONSTRAINT chk_acct_cash_accounts_ownership
  CHECK (ownership IN ('Business', 'Personal'));

-- Main Cash Journal (~38k rows)
CREATE TABLE acct_cash_transactions (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_code        VARCHAR(20) UNIQUE NOT NULL,
    transaction_date        DATE NOT NULL,
    item_id                 UUID REFERENCES acct_items(id),
    description             TEXT NOT NULL,
    quantity                DECIMAL(12,4) NOT NULL DEFAULT 1,
    unit_price              DECIMAL(12,2) NOT NULL,
    amount                  DECIMAL(12,2) NOT NULL,
    line_type               VARCHAR(20) NOT NULL,
    account_id              UUID NOT NULL REFERENCES acct_accounts(id),
    cash_account_id         UUID REFERENCES acct_cash_accounts(id),
    outlet_id               UUID REFERENCES outlets(id),
    reimbursement_batch_id  VARCHAR(30),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_cash_tx_date ON acct_cash_transactions(transaction_date);
CREATE INDEX idx_cash_tx_line_type ON acct_cash_transactions(line_type);
CREATE INDEX idx_cash_tx_account ON acct_cash_transactions(account_id);
CREATE INDEX idx_cash_tx_cash_account ON acct_cash_transactions(cash_account_id);
CREATE INDEX idx_cash_tx_outlet ON acct_cash_transactions(outlet_id);
CREATE INDEX idx_cash_tx_item ON acct_cash_transactions(item_id);

ALTER TABLE acct_cash_transactions ADD CONSTRAINT chk_cash_tx_line_type
  CHECK (line_type IN ('ASSET', 'INVENTORY', 'EXPENSE', 'SALES', 'COGS', 'LIABILITY', 'CAPITAL', 'DRAWING'));

-- Reimbursement Requests (~1k rows)
CREATE TABLE acct_reimbursement_requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id        VARCHAR(30),
    expense_date    DATE NOT NULL,
    item_id         UUID REFERENCES acct_items(id),
    description     TEXT NOT NULL,
    qty             DECIMAL(12,4) NOT NULL DEFAULT 1,
    unit_price      DECIMAL(12,2) NOT NULL,
    amount          DECIMAL(12,2) NOT NULL,
    line_type       VARCHAR(20) NOT NULL,
    account_id      UUID NOT NULL REFERENCES acct_accounts(id),
    status          VARCHAR(10) NOT NULL DEFAULT 'Draft',
    requester       VARCHAR(100) NOT NULL,
    receipt_link    TEXT,
    posted_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE acct_reimbursement_requests ADD CONSTRAINT chk_acct_reimb_status
  CHECK (status IN ('Draft', 'Ready', 'Posted'));

CREATE INDEX idx_reimb_status ON acct_reimbursement_requests(status);
CREATE INDEX idx_reimb_batch ON acct_reimbursement_requests(batch_id);

ALTER TABLE acct_reimbursement_requests ADD CONSTRAINT chk_reimb_line_type
  CHECK (line_type IN ('ASSET', 'INVENTORY', 'EXPENSE', 'SALES', 'COGS', 'LIABILITY', 'CAPITAL', 'DRAWING'));

-- Sales Daily Summary
CREATE TABLE acct_sales_daily_summaries (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sales_date          DATE NOT NULL,
    channel             VARCHAR(30) NOT NULL,
    payment_method      VARCHAR(30) NOT NULL,
    gross_sales         DECIMAL(12,2) NOT NULL,
    discount_amount     DECIMAL(12,2) NOT NULL DEFAULT 0,
    net_sales           DECIMAL(12,2) NOT NULL,
    cash_account_id     UUID NOT NULL REFERENCES acct_cash_accounts(id),
    outlet_id           UUID REFERENCES outlets(id),
    source              VARCHAR(10) NOT NULL DEFAULT 'manual',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(sales_date, channel, payment_method, outlet_id)
);

ALTER TABLE acct_sales_daily_summaries ADD CONSTRAINT chk_acct_sales_source
  CHECK (source IN ('pos', 'manual'));

-- Payroll Entries
CREATE TABLE acct_payroll_entries (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payroll_date        DATE NOT NULL,
    period_type         VARCHAR(10) NOT NULL,
    period_ref          VARCHAR(30),
    employee_name       VARCHAR(100) NOT NULL,
    gross_pay           DECIMAL(12,2) NOT NULL,
    payment_method      VARCHAR(20) NOT NULL,
    cash_account_id     UUID NOT NULL REFERENCES acct_cash_accounts(id),
    outlet_id           UUID REFERENCES outlets(id),
    posted_at           TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE acct_payroll_entries ADD CONSTRAINT chk_acct_payroll_period
  CHECK (period_type IN ('Daily', 'Weekly', 'Monthly'));
