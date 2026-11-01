-- 27. Пик онлайна против пика предыдущего эфира того же стримера.
-- Только площадка 'VK Play Live'; prev_peak — пик предыдущего по started_at эфира того
-- же стримера, delta — их разность; первый эфир каждого стримера не выводится.
--
-- Колонки: streamer, followers, started_at, title, peak_viewers, prev_peak, delta
-- Порядок: followers по убыванию, внутри стримера started_at по возрастанию
-- Строк:   592

SELECT x.streamer,
       x.followers,
       x.started_at,
       x.title,
       x.peak_viewers,
       x.prev_peak,
       x.peak_viewers - x.prev_peak AS delta
FROM (
    SELECT sr.streamer_id,
           sr.nickname AS streamer,
           sr.followers,
           st.started_at,
           st.title,
           st.peak_viewers,
           lag(st.peak_viewers) OVER (PARTITION BY st.streamer_id
                                      ORDER BY st.started_at) AS prev_peak
    FROM streams st
    JOIN streamers sr ON sr.streamer_id = st.streamer_id
    JOIN platforms pl ON pl.platform_id = sr.platform_id
    WHERE pl.name = 'VK Play Live'
) x
WHERE x.prev_peak IS NOT NULL
ORDER BY x.followers DESC, x.streamer_id, x.started_at;
