-- Удаляем уникальный индекс на chat_id + type (больше не нужен)
DROP INDEX IF EXISTS idx_xi_modes_chat_type;

-- Удаляем индекс на chat_id
DROP INDEX IF EXISTS idx_xi_modes_chat_id;

-- Удаляем колонку chat_id (режимы теперь глобальные)
ALTER TABLE xi_modes DROP COLUMN IF EXISTS chat_id;

-- Удаляем колонку prompt (промпт теперь только в config JSON)
ALTER TABLE xi_modes DROP COLUMN IF EXISTS prompt;

-- Создаем уникальный индекс на type чтобы ключи режимов были уникальны
-- (при создании нового режима с тем же ключом - делаем новую версию)
-- Не создаем уникальный индекс - разрешаем дублирование, но при получении делаем DISTINCT по type с ORDER BY created_at DESC

-- Обновляем индекс на grade для фильтрации по грейдам
CREATE INDEX IF NOT EXISTS idx_xi_modes_grade ON xi_modes(grade);