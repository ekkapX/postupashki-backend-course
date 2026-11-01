-- 16. Десятка самых банимых героев.
-- bans — строки match_bans, picks — строки match_players, bans_per_pick =
-- round(bans::numeric / picks, 2); первые десять берутся после сортировки.
--
-- Колонки: hero, bans, picks, bans_per_pick
-- Порядок: bans по убыванию, при равенстве hero по возрастанию
-- Строк:   10

SELECT h.name AS hero,
       b.bans,
       p.picks,
       round(b.bans::numeric / p.picks, 2) AS bans_per_pick
FROM heroes h
JOIN (SELECT hero_id, count(*) AS bans  FROM match_bans    GROUP BY hero_id) b ON b.hero_id = h.hero_id
JOIN (SELECT hero_id, count(*) AS picks FROM match_players GROUP BY hero_id) p ON p.hero_id = h.hero_id
ORDER BY b.bans DESC, h.name
LIMIT 10;
