-- w7/m392: which agent profiles a workspace's admins allow (Devin: "admins
-- control which agents are available"), and each data-use acknowledgement an
-- admin gave. disabled_agents lists profile ids turned off, so a profile added
-- later starts on unless it needs an acknowledgement. agent_data_use_acks maps
-- a profile id to {"at": RFC 3339, "by": subject} for profiles whose provider
-- may train on what sessions send (ADR108: muse, Meta's contributor tier),
-- which stay off until an admin acknowledges that.
ALTER TABLE agent_environments
    ADD COLUMN disabled_agents     text[] NOT NULL DEFAULT '{}' CHECK (cardinality(disabled_agents) <= 32),
    ADD COLUMN agent_data_use_acks jsonb  NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(agent_data_use_acks) = 'object');
