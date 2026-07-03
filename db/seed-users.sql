-- Insucar — real-like login users (prototype). Passwords are SHA-256 (demo-grade).
-- Run after schema.sql + seed.sql + schema-v4-auth.sql.

-- Agents (staff) get agent_id + password
UPDATE staff SET agent_id='OP-1001',  password_hash='f76899a1880b817e64de54359578295fcc05c2687fd09da6d025723be9719f70' WHERE email='operator@insucar.demo';
UPDATE staff SET agent_id='SUP-2001', password_hash='aded6f75dd64a3f153d5b6be2fc91d5b94f17f606643b13c2b69200782d53daa' WHERE email='supervisor@insucar.demo';
UPDATE staff SET agent_id='PO-3001',  password_hash='386e4f8f7f73b2cd2b5f1102b6b16ac7e9ef0b92c0d270c71bf089ef81568ca6' WHERE email='po@insucar.demo';

-- Customers get passwords (login by email)
UPDATE customers SET password_hash='c2adedf84b3ae5da71f05d2bed197456ed9ef294bf1659bdc433f35720b867ad' WHERE email='claire.martin@example.fr';
UPDATE customers SET password_hash='69b811db77937ff051354b2cfd151ab816836c3fc66c975f9eee0cac6d0e089a' WHERE email='john.smith@example.co.uk';
UPDATE customers SET password_hash='9a018a55046500e5d84667a33a76428ceb1bdabd694a3fc5e5fc36c4553d1f50' WHERE email='lukas.mueller@example.de';
