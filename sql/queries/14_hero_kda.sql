-- 14. Средние убийства, смерти и помощь у ходовых героев.
-- Только герои с games >= 420; средние — round(avg(x), 2) по numeric, два знака.
--
-- Колонки: hero, games, avg_kills, avg_deaths, avg_assists
-- Порядок: avg_kills (округлённое) по убыванию, при равенстве hero по возрастанию
-- Строк:   29

SELECT h.name AS hero,
       count(*) AS games,
       round(avg(mp.kills)::numeric, 2)   AS avg_kills,
       round(avg(mp.deaths)::numeric, 2)  AS avg_deaths,
       round(avg(mp.assists)::numeric, 2) AS avg_assists
FROM match_players mp
JOIN heroes h ON h.hero_id = mp.hero_id
GROUP BY h.hero_id, h.name
HAVING count(*) >= 420
ORDER BY round(avg(mp.kills)::numeric, 2) DESC, h.name;
