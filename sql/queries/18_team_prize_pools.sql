-- 18. Призовые турниров, до которых добрались игроки команды.
-- Турнир в зачёт один раз, если игрок сыграл его матч в составе: joined_at <=
-- started_at::date < left_at (пустой left_at — сейчас); команды без турниров не выводятся.
--
-- Колонки: team, tag, tournaments, prize_total
-- Порядок: prize_total по убыванию, при равенстве team по возрастанию
-- Строк:   22

WITH pairs AS (
    SELECT DISTINCT tm.team_id, m.tournament_id
    FROM matches m
    JOIN match_players mp ON mp.match_id = m.match_id
    JOIN team_members tm  ON tm.player_id = mp.player_id
                         AND (m.started_at AT TIME ZONE 'UTC')::date >= tm.joined_at
                         AND (tm.left_at IS NULL
                              OR (m.started_at AT TIME ZONE 'UTC')::date < tm.left_at)
    WHERE m.tournament_id IS NOT NULL
)
SELECT t.name AS team,
       t.tag,
       count(*) AS tournaments,
       sum(tr.prize_pool) AS prize_total
FROM pairs pr
JOIN teams t        ON t.team_id = pr.team_id
JOIN tournaments tr ON tr.tournament_id = pr.tournament_id
GROUP BY t.team_id, t.name, t.tag
ORDER BY sum(tr.prize_pool) DESC, t.name;
