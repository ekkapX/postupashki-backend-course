-- 20. Герои, которых на патче банили чаще, чем брали.
-- Патч — matches.patch_id; в ответе пары с bans строго больше picks, diff = bans - picks.
--
-- Колонки: version, hero, bans, picks, diff
-- Порядок: diff по убыванию, затем hero по возрастанию, затем version по возрастанию
-- Строк:   99

WITH b AS (
    SELECT m.patch_id, mb.hero_id, count(*) AS bans
    FROM match_bans mb
    JOIN matches m ON m.match_id = mb.match_id
    GROUP BY m.patch_id, mb.hero_id
), p AS (
    SELECT m.patch_id, mp.hero_id, count(*) AS picks
    FROM match_players mp
    JOIN matches m ON m.match_id = mp.match_id
    GROUP BY m.patch_id, mp.hero_id
)
SELECT pt.version,
       h.name AS hero,
       b.bans,
       COALESCE(p.picks, 0) AS picks,
       b.bans - COALESCE(p.picks, 0) AS diff
FROM b
LEFT JOIN p ON p.patch_id = b.patch_id AND p.hero_id = b.hero_id
JOIN patches pt ON pt.patch_id = b.patch_id
JOIN heroes h   ON h.hero_id = b.hero_id
WHERE b.bans > COALESCE(p.picks, 0)
ORDER BY diff DESC, h.name, pt.version;
