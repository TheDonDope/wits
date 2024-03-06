ALTER TABLE auth.users
ADD COLUMN account UUID,
ADD CONSTRAINT users_account_fkey FOREIGN KEY (account) REFERENCES accounts(id);