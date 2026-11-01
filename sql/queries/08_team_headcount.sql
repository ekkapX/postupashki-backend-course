-- 08. Сколько в команде игроков и сколько среди них стримеров.
-- Считаются только незакрытые периоды; команда без состава остаётся
-- в ответе с нулями, стримеры без player_id не учитываются.
--
-- Колонки: name, tag, players_now, streamers_now
-- Порядок: players_now по убыванию, при равенстве name по возрастанию
-- Строк:   24

SELECT t.name,
       t.tag,
       count(DISTINCT tm.player_id) AS players_now,
       count(DISTINCT s.streamer_id) AS streamers_now
FROM teams t
LEFT JOIN team_members tm ON tm.team_id = t.team_id AND tm.left_at IS NULL
LEFT JOIN streamers s     ON s.player_id = tm.player_id
GROUP BY t.team_id, t.name, t.tag
ORDER BY players_now DESC, t.name;
