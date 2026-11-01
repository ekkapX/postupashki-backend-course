-- 30. Стоимость сборки из базовых составляющих против собственной цены предмета.
-- Берутся предметы со строками в item_recipe, сборка раскрывается до базовых (тех, у кого
-- строк в item_recipe нет, свитки тоже), количества по пути перемножаются; diff = parts_cost - cost.
--
-- Колонки: name, cost, parts_cost, base_parts, diff
-- Порядок: diff по убыванию, при равенстве name по возрастанию
-- Строк:   77

WITH RECURSIVE x AS (
    SELECT r.item_id AS root_id, r.component_id, r.quantity AS qty
    FROM item_recipe r
    UNION ALL
    SELECT x.root_id, r.component_id, x.qty * r.quantity
    FROM x
    JOIN item_recipe r ON r.item_id = x.component_id
)
SELECT i.name,
       i.cost,
       sum(c.cost * x.qty)::bigint AS parts_cost,
       sum(x.qty)::bigint AS base_parts,
       (sum(c.cost * x.qty) - i.cost)::bigint AS diff
FROM x
JOIN items i ON i.item_id = x.root_id
JOIN items c ON c.item_id = x.component_id
WHERE NOT EXISTS (SELECT 1 FROM item_recipe r WHERE r.item_id = x.component_id)
GROUP BY i.item_id, i.name, i.cost
ORDER BY (sum(c.cost * x.qty) - i.cost) DESC, i.name;
