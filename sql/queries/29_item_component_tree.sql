-- 29. Полное дерево сборки Guardian Greaves, Bloodthorn, Harpoon и Hurricane Pike.
-- Строка на каждый путь вниз, сам корень не выводится; lvl с 1, qty — quantity последнего
-- шага, qty_total — произведение quantity по пути, path — имена от корня через ' > '.
--
-- Колонки: root_item, lvl, component, qty, qty_total, path
-- Порядок: root_item по возрастанию, затем path по возрастанию
-- Строк:   34

WITH RECURSIVE tree AS (
    SELECT i.name AS root_item,
           1 AS lvl,
           c.name AS component,
           r.quantity AS qty,
           r.quantity AS qty_total,
           r.component_id,
           i.name || ' > ' || c.name AS path
    FROM items i
    JOIN item_recipe r ON r.item_id = i.item_id
    JOIN items c ON c.item_id = r.component_id
    WHERE i.name IN ('Guardian Greaves', 'Bloodthorn', 'Harpoon', 'Hurricane Pike')
    UNION ALL
    SELECT t.root_item,
           t.lvl + 1,
           c.name,
           r.quantity,
           t.qty_total * r.quantity,
           r.component_id,
           t.path || ' > ' || c.name
    FROM tree t
    JOIN item_recipe r ON r.item_id = t.component_id
    JOIN items c ON c.item_id = r.component_id
)
SELECT root_item, lvl, component, qty, qty_total, path
FROM tree
ORDER BY root_item, path;
