-- 17. Доля побед светлой стороны по патчам.
-- Патч без единого матча не выводится; radiant_winrate — процент,
-- round(100.0 * radiant_wins / matches, 2).
--
-- Колонки: version, matches, radiant_wins, radiant_winrate
-- Порядок: radiant_winrate (округлённое) по убыванию, при равенстве version по возрастанию
-- Строк:   12

SELECT p.version,
       count(*) AS matches,
       count(*) FILTER (WHERE m.winner = 'radiant') AS radiant_wins,
       round(100.0 * count(*) FILTER (WHERE m.winner = 'radiant') / count(*), 2) AS radiant_winrate
FROM matches m
JOIN patches p ON p.patch_id = m.patch_id
GROUP BY p.patch_id, p.version
ORDER BY round(100.0 * count(*) FILTER (WHERE m.winner = 'radiant') / count(*), 2) DESC, p.version;
