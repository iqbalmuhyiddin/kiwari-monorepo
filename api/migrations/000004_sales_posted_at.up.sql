-- Add posted_at to acct_sales_daily_summaries for sales posting workflow.
-- Matches the pattern in acct_payroll_entries.posted_at.
ALTER TABLE acct_sales_daily_summaries ADD COLUMN posted_at TIMESTAMPTZ;
