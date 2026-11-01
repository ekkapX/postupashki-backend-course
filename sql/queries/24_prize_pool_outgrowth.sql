-- 24. Турниры, чей призовой фонд больше суммы призовых всех предыдущих.
-- Предыдущие — со строго меньшим started_at, сравнение строгое; у первого турнира
-- prev_total и prev_count нули, не NULL; matches_played — число матчей турнира.
--
-- Колонки: tournament, started_at, prize_pool, prev_total, prev_count, matches_played
-- Порядок: started_at по возрастанию
-- Строк:   2

SELECT t.name AS tournament,
       t.started_at,
       t.prize_pool,
       pr.prev_total,
       pr.prev_count,
       (SELECT count(*) FROM matches m WHERE m.tournament_id = t.tournament_id) AS matches_played
FROM tournaments t
CROSS JOIN LATERAL (
    SELECT COALESCE(sum(x.prize_pool), 0)::bigint AS prev_total,
           count(*) AS prev_count
    FROM tournaments x
    WHERE x.started_at < t.started_at
) pr
WHERE t.prize_pool > pr.prev_total
ORDER BY t.started_at;
