-- Extend instruments table with products, competitors, and supply chain information
ALTER TABLE instruments ADD COLUMN products TEXT DEFAULT '[]';
ALTER TABLE instruments ADD COLUMN competitors TEXT DEFAULT '[]';
ALTER TABLE instruments ADD COLUMN supply_chain TEXT DEFAULT '[]';
