-- 22. Стримеры с подписчиками выше среднего по своей площадке.
-- Строго больше среднего по всем стримерам площадки, сам стример в среднее входит;
-- platform_avg до 2 знаков; team — тег действующей команды, иначе NULL.
--
-- Колонки: streamer, platform, followers, platform_avg, team
-- Порядок: followers по убыванию, при равенстве streamer по возрастанию
-- Строк:   16

SELECT s.nickname AS streamer,
       pl.name AS platform,
       s.followers,
       round(a.avg_followers, 2) AS platform_avg,
       t.tag AS team
FROM streamers s
JOIN platforms pl ON pl.platform_id = s.platform_id
JOIN (
    SELECT platform_id, avg(followers::numeric) AS avg_followers
    FROM streamers
    GROUP BY platform_id
) a ON a.platform_id = s.platform_id
LEFT JOIN team_members tm ON tm.player_id = s.player_id AND tm.left_at IS NULL
LEFT JOIN teams t ON t.team_id = tm.team_id
WHERE s.followers > a.avg_followers
ORDER BY s.followers DESC, s.nickname;
