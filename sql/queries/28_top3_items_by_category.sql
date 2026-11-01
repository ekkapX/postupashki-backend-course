-- 28. Три самых дорогих предмета в каждой категории лавки.
-- Категория: 'recipe' при is_recipe, иначе 'assembled' при строках в item_recipe.item_id,
-- иначе 'basic'; place 1-3 по cost убыв., при равной цене по name; purchases — строк match_items, иначе 0.
--
-- Колонки: category, place, name, cost, purchases
-- Порядок: category по возрастанию, затем place по возрастанию
-- Строк:   9

WITH c AS (
    SELECT i.item_id, i.name, i.cost,
           CASE
               WHEN i.is_recipe THEN 'recipe'
               WHEN EXISTS (SELECT 1 FROM item_recipe r WHERE r.item_id = i.item_id) THEN 'assembled'
               ELSE 'basic'
           END AS category
    FROM items i
), ranked AS (
    SELECT c.*,
           row_number() OVER (PARTITION BY category ORDER BY cost DESC, name) AS place
    FROM c
)
SELECT ranked.category,
       ranked.place,
       ranked.name,
       ranked.cost,
       (SELECT count(*) FROM match_items mi WHERE mi.item_id = ranked.item_id) AS purchases
FROM ranked
WHERE ranked.place <= 3
ORDER BY ranked.category, ranked.place;
