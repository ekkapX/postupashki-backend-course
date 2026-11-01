-- 15. Сколько донатов собрал каждый стример.
-- В ответе все стримеры: без донатов donations = 0 и total_amount = 0.00;
-- team — команда с left_at IS NULL, иначе пусто.
--
-- Колонки: streamer, platform, team, donations, total_amount
-- Порядок: total_amount по убыванию, при равенстве streamer по возрастанию
-- Строк:   40

SELECT s.nickname AS streamer,
       pl.name AS platform,
       t.name  AS team,
       COALESCE(d.donations, 0) AS donations,
       round(COALESCE(d.total, 0)::numeric, 2) AS total_amount
FROM streamers s
JOIN platforms pl ON pl.platform_id = s.platform_id
LEFT JOIN (
    SELECT st.streamer_id, count(*) AS donations, sum(sd.amount) AS total
    FROM streams st
    JOIN stream_donations sd ON sd.stream_id = st.stream_id
    GROUP BY st.streamer_id
) d ON d.streamer_id = s.streamer_id
LEFT JOIN team_members tm ON tm.player_id = s.player_id AND tm.left_at IS NULL
LEFT JOIN teams t ON t.team_id = tm.team_id
ORDER BY round(COALESCE(d.total, 0)::numeric, 2) DESC, s.nickname;
