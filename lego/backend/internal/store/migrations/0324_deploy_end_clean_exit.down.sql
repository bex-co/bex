UPDATE deploys SET end_exit_code = NULL WHERE end_exit_code = 0;
ALTER TABLE deploys ADD CONSTRAINT deploys_end_exit_code_check
    CHECK (end_exit_code IS NULL OR end_exit_code <> 0);
