-- 07. Действующие составы: кто сейчас за кого играет.
-- Только незакрытые периоды (left_at IS NULL); игроки без команды не выводятся.
--
-- Колонки: nickname, country, mmr, team_name, joined_at
-- Порядок: joined_at по возрастанию, при равенстве nickname по возрастанию
-- Строк:   86

SELECT p.nickname, p.country, p.mmr, t.name AS team_name, tm.joined_at
FROM team_members tm
JOIN players p ON p.player_id = tm.player_id
JOIN teams t   ON t.team_id = tm.team_id
WHERE tm.left_at IS NULL
ORDER BY tm.joined_at, p.nickname;
