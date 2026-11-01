-- 21. Турнирные выступления с KDA выше шестикратного среднего.
-- KDA = (kills + assists)::numeric / GREATEST(deaths, 1); порог — строго больше шести
-- средних KDA по всей match_players; только матчи турниров; kda округлить до 2 знаков.
--
-- Колонки: match_id, tournament, player, hero, kda
-- Порядок: kda по убыванию, при равенстве match_id, затем slot
-- Строк:   60

WITH th AS (
    SELECT 6 * avg((kills + assists)::numeric / GREATEST(deaths, 1)) AS thr
    FROM match_players
)
SELECT mp.match_id,
       tr.name AS tournament,
       p.nickname AS player,
       h.name AS hero,
       round((mp.kills + mp.assists)::numeric / GREATEST(mp.deaths, 1), 2) AS kda
FROM match_players mp
JOIN matches m      ON m.match_id = mp.match_id
JOIN tournaments tr ON tr.tournament_id = m.tournament_id
JOIN players p      ON p.player_id = mp.player_id
JOIN heroes h       ON h.hero_id = mp.hero_id
CROSS JOIN th
WHERE (mp.kills + mp.assists)::numeric / GREATEST(mp.deaths, 1) > th.thr
ORDER BY kda DESC, mp.match_id, mp.slot;
