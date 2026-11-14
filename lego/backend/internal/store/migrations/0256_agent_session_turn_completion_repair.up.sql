-- w6/126: CompleteAgentSessionTurn re-stamped completed_at with now() on every
-- transcript harvest, including the Completer's late one when it reaps the
-- idle sandbox (BEX_AGENT_SANDBOX_IDLE_TTL, 2h by default). A session's last
-- turn therefore read its real length plus that delay. The function now keeps
-- the first value; this repairs the turns it already moved.
--
-- Only turns whose end is evident are repaired: the session's last turn, whose
-- transcript rows were all stored in the same second (one harvest at the
-- turn's end, no live tee), and whose completed_at sits more than 90 minutes
-- after that harvest. That harvest is the turn's end. Any other turn is left
-- as it is.
UPDATE agent_session_turns t
SET completed_at = h.harvested
FROM (
  SELECT session_id, turn, min(created_at) AS harvested
  FROM agent_session_transcripts
  GROUP BY session_id, turn
  HAVING date_trunc('second', min(created_at)) = date_trunc('second', max(created_at))
) h
WHERE t.session_id = h.session_id AND t.turn = h.turn
  AND t.completed_at > h.harvested + interval '90 minutes'
  AND NOT EXISTS (
    SELECT 1 FROM agent_session_turns n WHERE n.session_id = t.session_id AND n.turn > t.turn
  );
