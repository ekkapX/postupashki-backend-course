-- 23. Игроки, поигравшие больше чем за одну команду.
-- Считаются различные team_id по всем периодам, строго больше одного; current_team —
-- название команды с пустым left_at, current_team и streamer могут быть NULL.
--
-- Колонки: player, country, teams_count, current_team, streamer
-- Порядок: teams_count по убыванию, при равенстве player по возрастанию
-- Строк:   25

SELECT p.nickname AS player,
       p.country,
       count(DISTINCT tm.team_id) AS teams_count,
       ct.name AS current_team,
       sr.nickname AS streamer
FROM players p
JOIN team_members tm ON tm.player_id = p.player_id
LEFT JOIN team_members cm ON cm.player_id = p.player_id AND cm.left_at IS NULL
LEFT JOIN teams ct ON ct.team_id = cm.team_id
LEFT JOIN streamers sr ON sr.player_id = p.player_id
GROUP BY p.player_id, p.nickname, p.country, ct.name, sr.nickname
HAVING count(DISTINCT tm.team_id) > 1
ORDER BY teams_count DESC, p.nickname;
