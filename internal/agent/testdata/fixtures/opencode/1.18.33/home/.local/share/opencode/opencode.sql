PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
CREATE TABLE `workspace` (
          `id` text PRIMARY KEY,
          `type` text NOT NULL,
          `name` text DEFAULT '' NOT NULL,
          `branch` text,
          `directory` text,
          `extra` text,
          `project_id` text NOT NULL,
          `time_used` integer NOT NULL,
          CONSTRAINT `fk_workspace_project_id_project_id_fk` FOREIGN KEY (`project_id`) REFERENCES `project`(`id`) ON DELETE CASCADE
        );
CREATE TABLE `data_migration` (
          `name` text PRIMARY KEY,
          `time_completed` integer NOT NULL
        );
CREATE TABLE `account_state` (
          `id` integer PRIMARY KEY,
          `active_account_id` text,
          `active_org_id` text,
          CONSTRAINT `fk_account_state_active_account_id_account_id_fk` FOREIGN KEY (`active_account_id`) REFERENCES `account`(`id`) ON DELETE SET NULL
        );
CREATE TABLE `account` (
          `id` text PRIMARY KEY,
          `email` text NOT NULL,
          `url` text NOT NULL,
          `access_token` text NOT NULL,
          `refresh_token` text NOT NULL,
          `token_expiry` integer,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL
        );
CREATE TABLE `control_account` (
          `email` text NOT NULL,
          `url` text NOT NULL,
          `access_token` text NOT NULL,
          `refresh_token` text NOT NULL,
          `token_expiry` integer,
          `active` integer NOT NULL,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL,
          CONSTRAINT `control_account_pk` PRIMARY KEY(`email`, `url`)
        );
CREATE TABLE `credential` (
          `id` text PRIMARY KEY,
          `integration_id` text,
          `label` text NOT NULL,
          `value` text NOT NULL,
          `connector_id` text,
          `method_id` text,
          `active` integer,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL
        );
CREATE TABLE `event_sequence` (
          `aggregate_id` text PRIMARY KEY,
          `seq` integer NOT NULL,
          `owner_id` text
        );
INSERT INTO event_sequence VALUES('ses_f0de500f8ffeXvsiYJDOg5JtrU',27,NULL);
CREATE TABLE `event` (
          `id` text PRIMARY KEY,
          `aggregate_id` text NOT NULL,
          `seq` integer NOT NULL,
          `type` text NOT NULL,
          `data` text NOT NULL,
          CONSTRAINT `fk_event_aggregate_id_event_sequence_aggregate_id_fk` FOREIGN KEY (`aggregate_id`) REFERENCES `event_sequence`(`aggregate_id`) ON DELETE CASCADE
        );
INSERT INTO event VALUES('evt_0f21aff080016p0jqt2Phdo56r','ses_f0de500f8ffeXvsiYJDOg5JtrU',0,'session.created.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","info":{"id":"ses_f0de500f8ffeXvsiYJDOg5JtrU","slug":"sunny-nebula","projectID":"global","directory":"/work/proj","path":"tmp/lazyagents-fixture-1634861929/work/proj","cost":0,"tokens":{"input":0,"output":0,"reasoning":0,"cache":{"read":0,"write":0}},"title":"New session - 2026-09-30T11:37:30.632Z","version":"1.18.33","time":{"created":1790768250632,"updated":1790768250632},"permission":[{"permission":"question","pattern":"*","action":"deny"},{"permission":"plan_enter","pattern":"*","action":"deny"},{"permission":"plan_exit","pattern":"*","action":"deny"}]}}');
INSERT INTO event VALUES('evt_0f21aff30001OWO5vP3uLLZIL2','ses_f0de500f8ffeXvsiYJDOg5JtrU',1,'session.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","info":{"id":"ses_f0de500f8ffeXvsiYJDOg5JtrU","slug":"sunny-nebula","projectID":"global","directory":"/work/proj","path":"tmp/lazyagents-fixture-1634861929/work/proj","cost":0,"tokens":{"input":0,"output":0,"reasoning":0,"cache":{"read":0,"write":0}},"title":"New session - 2026-09-30T11:37:30.632Z","agent":"build","model":{"id":"fake-model","providerID":"fake","variant":"default"},"version":"1.18.33","time":{"created":1790768250632,"updated":1790768250670},"permission":[{"permission":"question","pattern":"*","action":"deny"},{"permission":"plan_enter","pattern":"*","action":"deny"},{"permission":"plan_exit","pattern":"*","action":"deny"}]}}');
INSERT INTO event VALUES('evt_0f21aff34001I6ZqEPBrU6ZAA4','ses_f0de500f8ffeXvsiYJDOg5JtrU',2,'message.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","info":{"id":"msg_0f21aff2e001y84CAhBLtC5Dm9","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","role":"user","time":{"created":1790768250670},"agent":"build","model":{"providerID":"fake","modelID":"fake-model"}}}');
INSERT INTO event VALUES('evt_0f21b00fa001lV9DbRgQS07VGJ','ses_f0de500f8ffeXvsiYJDOg5JtrU',3,'message.part.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","part":{"id":"prt_0f21aff33001a2Ikz6KYrhKnir","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","messageID":"msg_0f21aff2e001y84CAhBLtC5Dm9","type":"text","text":"\"run echo fixture please\""},"time":1790768251129}');
INSERT INTO event VALUES('evt_0f21b00fd0016sm94SpUAFObBJ','ses_f0de500f8ffeXvsiYJDOg5JtrU',4,'session.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","info":{"id":"ses_f0de500f8ffeXvsiYJDOg5JtrU","slug":"sunny-nebula","projectID":"global","directory":"/work/proj","path":"tmp/lazyagents-fixture-1634861929/work/proj","cost":0,"tokens":{"input":0,"output":0,"reasoning":0,"cache":{"read":0,"write":0}},"title":"New session - 2026-09-30T11:37:30.632Z","agent":"build","model":{"id":"fake-model","providerID":"fake","variant":"default"},"version":"1.18.33","time":{"created":1790768250632,"updated":1790768251132},"permission":[{"permission":"question","pattern":"*","action":"deny"},{"permission":"plan_enter","pattern":"*","action":"deny"},{"permission":"plan_exit","pattern":"*","action":"deny"}]}}');
INSERT INTO event VALUES('evt_0f21b0104001yoyBG6DG22f1f4','ses_f0de500f8ffeXvsiYJDOg5JtrU',5,'message.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","info":{"id":"msg_0f21b0104001XDVLEwtN0JiIZk","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","role":"assistant","time":{"created":1790768251140},"parentID":"msg_0f21aff2e001y84CAhBLtC5Dm9","modelID":"fake-model","providerID":"fake","mode":"build","agent":"build","path":{"cwd":"/work/proj","root":"/"},"cost":0,"tokens":{"input":0,"output":0,"reasoning":0,"cache":{"read":0,"write":0}}}}');
INSERT INTO event VALUES('evt_0f21b011c001K1Yutmn1oMXeTf','ses_f0de500f8ffeXvsiYJDOg5JtrU',6,'session.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","info":{"id":"ses_f0de500f8ffeXvsiYJDOg5JtrU","slug":"sunny-nebula","projectID":"global","directory":"/work/proj","path":"tmp/lazyagents-fixture-1634861929/work/proj","summary":{"additions":0,"deletions":0,"files":0},"cost":0,"tokens":{"input":0,"output":0,"reasoning":0,"cache":{"read":0,"write":0}},"title":"New session - 2026-09-30T11:37:30.632Z","agent":"build","model":{"id":"fake-model","providerID":"fake","variant":"default"},"version":"1.18.33","time":{"created":1790768250632,"updated":1790768251163},"permission":[{"permission":"question","pattern":"*","action":"deny"},{"permission":"plan_enter","pattern":"*","action":"deny"},{"permission":"plan_exit","pattern":"*","action":"deny"}]}}');
INSERT INTO event VALUES('evt_0f21b011e001FX5tedrtfNogjc','ses_f0de500f8ffeXvsiYJDOg5JtrU',7,'message.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","info":{"id":"msg_0f21aff2e001y84CAhBLtC5Dm9","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","role":"user","time":{"created":1790768250670},"summary":{"diffs":[]},"agent":"build","model":{"providerID":"fake","modelID":"fake-model"}}}');
INSERT INTO event VALUES('evt_0f21b013f0011r9aOE89ZxQmxm','ses_f0de500f8ffeXvsiYJDOg5JtrU',8,'session.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","info":{"id":"ses_f0de500f8ffeXvsiYJDOg5JtrU","slug":"sunny-nebula","projectID":"global","directory":"/work/proj","path":"tmp/lazyagents-fixture-1634861929/work/proj","summary":{"additions":0,"deletions":0,"files":0},"cost":0,"tokens":{"input":0,"output":0,"reasoning":0,"cache":{"read":0,"write":0}},"title":"The command printed fixture.","agent":"build","model":{"id":"fake-model","providerID":"fake","variant":"default"},"version":"1.18.33","time":{"created":1790768250632,"updated":1790768251163},"permission":[{"permission":"question","pattern":"*","action":"deny"},{"permission":"plan_enter","pattern":"*","action":"deny"},{"permission":"plan_exit","pattern":"*","action":"deny"}]}}');
INSERT INTO event VALUES('evt_0f21b02d3001ZCMIhn7pfAnxca','ses_f0de500f8ffeXvsiYJDOg5JtrU',9,'message.part.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","part":{"id":"prt_0f21b02d3001tOYcQAJkloLqkx","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","messageID":"msg_0f21b0104001XDVLEwtN0JiIZk","type":"step-start"},"time":1790768251603}');
INSERT INTO event VALUES('evt_0f21b02d5001kNm6T8TuJnR2dd','ses_f0de500f8ffeXvsiYJDOg5JtrU',10,'message.part.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","part":{"id":"prt_0f21b02d5001NcGUD0hmNfrPyr","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","messageID":"msg_0f21b0104001XDVLEwtN0JiIZk","type":"tool","callID":"call_1","tool":"bash","state":{"status":"pending","input":{},"raw":""}},"time":1790768251605}');
INSERT INTO event VALUES('evt_0f21b02db001UVDEWb3l0Sxq0B','ses_f0de500f8ffeXvsiYJDOg5JtrU',11,'message.part.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","part":{"id":"prt_0f21b02d5001NcGUD0hmNfrPyr","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","messageID":"msg_0f21b0104001XDVLEwtN0JiIZk","type":"tool","callID":"call_1","tool":"bash","state":{"status":"running","input":{"command":"echo fixture"},"time":{"start":1790768251611}}},"time":1790768251611}');
INSERT INTO event VALUES('evt_0f21b0303001GoDvlP3lJIb6AT','ses_f0de500f8ffeXvsiYJDOg5JtrU',12,'message.part.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","part":{"id":"prt_0f21b02d5001NcGUD0hmNfrPyr","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","messageID":"msg_0f21b0104001XDVLEwtN0JiIZk","type":"tool","callID":"call_1","tool":"bash","state":{"status":"running","input":{"command":"echo fixture"},"metadata":{"output":""},"time":{"start":1790768251611}}},"time":1790768251651}');
INSERT INTO event VALUES('evt_0f21b030e001J1GmhSnjVbDJ0Z','ses_f0de500f8ffeXvsiYJDOg5JtrU',13,'message.part.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","part":{"id":"prt_0f21b02d5001NcGUD0hmNfrPyr","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","messageID":"msg_0f21b0104001XDVLEwtN0JiIZk","type":"tool","callID":"call_1","tool":"bash","state":{"status":"running","input":{"command":"echo fixture"},"metadata":{"output":"fixture\n"},"time":{"start":1790768251611}}},"time":1790768251662}');
INSERT INTO event VALUES('evt_0f21b0312001uB4QXRaASBZLLf','ses_f0de500f8ffeXvsiYJDOg5JtrU',14,'message.part.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","part":{"id":"prt_0f21b02d5001NcGUD0hmNfrPyr","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","messageID":"msg_0f21b0104001XDVLEwtN0JiIZk","type":"tool","callID":"call_1","tool":"bash","state":{"status":"completed","input":{"command":"echo fixture"},"output":"fixture\n","title":"echo fixture","metadata":{"output":"fixture\n","exit":0,"truncated":false},"time":{"start":1790768251611,"end":1790768251666}}},"time":1790768251666}');
INSERT INTO event VALUES('evt_0f21b0315001tIf5NDYQrXHtQy','ses_f0de500f8ffeXvsiYJDOg5JtrU',15,'message.part.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","part":{"id":"prt_0f21b0315001UbHx9JXmviMyiM","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","messageID":"msg_0f21b0104001XDVLEwtN0JiIZk","type":"step-finish","reason":"tool-calls","cost":0,"tokens":{"total":128,"input":120,"output":8,"reasoning":0,"cache":{"read":0,"write":0}}},"time":1790768251669}');
INSERT INTO event VALUES('evt_0f21b0317001wO4Ot0c1cZbj5Z','ses_f0de500f8ffeXvsiYJDOg5JtrU',16,'message.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","info":{"id":"msg_0f21b0104001XDVLEwtN0JiIZk","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","role":"assistant","time":{"created":1790768251140},"parentID":"msg_0f21aff2e001y84CAhBLtC5Dm9","modelID":"fake-model","providerID":"fake","mode":"build","agent":"build","path":{"cwd":"/work/proj","root":"/"},"cost":0,"tokens":{"total":128,"input":120,"output":8,"reasoning":0,"cache":{"read":0,"write":0}},"finish":"tool-calls"}}');
INSERT INTO event VALUES('evt_0f21b0319001psOoaEbY46MdhU','ses_f0de500f8ffeXvsiYJDOg5JtrU',17,'message.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","info":{"id":"msg_0f21b0104001XDVLEwtN0JiIZk","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","role":"assistant","time":{"created":1790768251140,"completed":1790768251673},"parentID":"msg_0f21aff2e001y84CAhBLtC5Dm9","modelID":"fake-model","providerID":"fake","mode":"build","agent":"build","path":{"cwd":"/work/proj","root":"/"},"cost":0,"tokens":{"total":128,"input":120,"output":8,"reasoning":0,"cache":{"read":0,"write":0}},"finish":"tool-calls"}}');
INSERT INTO event VALUES('evt_0f21b031b001SpX6fGF5uMiFt2','ses_f0de500f8ffeXvsiYJDOg5JtrU',18,'message.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","info":{"id":"msg_0f21b031b0014ZxYbhc4DxjZrb","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","role":"assistant","time":{"created":1790768251675},"parentID":"msg_0f21aff2e001y84CAhBLtC5Dm9","modelID":"fake-model","providerID":"fake","mode":"build","agent":"build","path":{"cwd":"/work/proj","root":"/"},"cost":0,"tokens":{"input":0,"output":0,"reasoning":0,"cache":{"read":0,"write":0}}}}');
INSERT INTO event VALUES('evt_0f21b0325001z5kNMtMenPibKJ','ses_f0de500f8ffeXvsiYJDOg5JtrU',19,'session.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","info":{"id":"ses_f0de500f8ffeXvsiYJDOg5JtrU","slug":"sunny-nebula","projectID":"global","directory":"/work/proj","path":"tmp/lazyagents-fixture-1634861929/work/proj","summary":{"additions":0,"deletions":0,"files":0},"cost":0,"tokens":{"input":120,"output":8,"reasoning":0,"cache":{"read":0,"write":0}},"title":"The command printed fixture.","agent":"build","model":{"id":"fake-model","providerID":"fake","variant":"default"},"version":"1.18.33","time":{"created":1790768250632,"updated":1790768251685},"permission":[{"permission":"question","pattern":"*","action":"deny"},{"permission":"plan_enter","pattern":"*","action":"deny"},{"permission":"plan_exit","pattern":"*","action":"deny"}]}}');
INSERT INTO event VALUES('evt_0f21b0327001S7846qdnmk97jy','ses_f0de500f8ffeXvsiYJDOg5JtrU',20,'message.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","info":{"id":"msg_0f21aff2e001y84CAhBLtC5Dm9","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","role":"user","time":{"created":1790768250670},"summary":{"diffs":[]},"agent":"build","model":{"providerID":"fake","modelID":"fake-model"}}}');
INSERT INTO event VALUES('evt_0f21b0336001s9tliC3WFdQX4L','ses_f0de500f8ffeXvsiYJDOg5JtrU',21,'message.part.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","part":{"id":"prt_0f21b03360013A63H7yUuGZnIK","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","messageID":"msg_0f21b031b0014ZxYbhc4DxjZrb","type":"step-start"},"time":1790768251702}');
INSERT INTO event VALUES('evt_0f21b0337001fwx2UggFZbmM7v','ses_f0de500f8ffeXvsiYJDOg5JtrU',22,'message.part.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","part":{"id":"prt_0f21b0337001GtX6vvuhLhelY7","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","messageID":"msg_0f21b031b0014ZxYbhc4DxjZrb","type":"text","text":"","time":{"start":1790768251703}},"time":1790768251703}');
INSERT INTO event VALUES('evt_0f21b0339001gtQJub4G2Imb31','ses_f0de500f8ffeXvsiYJDOg5JtrU',23,'message.part.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","part":{"id":"prt_0f21b0337001GtX6vvuhLhelY7","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","messageID":"msg_0f21b031b0014ZxYbhc4DxjZrb","type":"text","text":"The command printed fixture.","time":{"start":1790768251703,"end":1790768251705}},"time":1790768251705}');
INSERT INTO event VALUES('evt_0f21b033a001b1P33ovLFG0Dyd','ses_f0de500f8ffeXvsiYJDOg5JtrU',24,'message.part.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","part":{"id":"prt_0f21b033a001VTlTYA9mpcWy8P","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","messageID":"msg_0f21b031b0014ZxYbhc4DxjZrb","type":"step-finish","reason":"stop","cost":0,"tokens":{"total":156,"input":50,"output":6,"reasoning":0,"cache":{"read":100,"write":0}}},"time":1790768251706}');
INSERT INTO event VALUES('evt_0f21b033b001XsqbMfmTtIIln3','ses_f0de500f8ffeXvsiYJDOg5JtrU',25,'message.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","info":{"id":"msg_0f21b031b0014ZxYbhc4DxjZrb","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","role":"assistant","time":{"created":1790768251675},"parentID":"msg_0f21aff2e001y84CAhBLtC5Dm9","modelID":"fake-model","providerID":"fake","mode":"build","agent":"build","path":{"cwd":"/work/proj","root":"/"},"cost":0,"tokens":{"total":156,"input":50,"output":6,"reasoning":0,"cache":{"read":100,"write":0}},"finish":"stop"}}');
INSERT INTO event VALUES('evt_0f21b033d0014LORJcKsG6UqdG','ses_f0de500f8ffeXvsiYJDOg5JtrU',26,'message.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","info":{"id":"msg_0f21b031b0014ZxYbhc4DxjZrb","sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","role":"assistant","time":{"created":1790768251675,"completed":1790768251709},"parentID":"msg_0f21aff2e001y84CAhBLtC5Dm9","modelID":"fake-model","providerID":"fake","mode":"build","agent":"build","path":{"cwd":"/work/proj","root":"/"},"cost":0,"tokens":{"total":156,"input":50,"output":6,"reasoning":0,"cache":{"read":100,"write":0}},"finish":"stop"}}');
INSERT INTO event VALUES('evt_0f21b034a001EUpaGhbdbHcio5','ses_f0de500f8ffeXvsiYJDOg5JtrU',27,'session.updated.1','{"sessionID":"ses_f0de500f8ffeXvsiYJDOg5JtrU","info":{"id":"ses_f0de500f8ffeXvsiYJDOg5JtrU","slug":"sunny-nebula","projectID":"global","directory":"/work/proj","path":"tmp/lazyagents-fixture-1634861929/work/proj","summary":{"additions":0,"deletions":0,"files":0},"cost":0,"tokens":{"input":170,"output":14,"reasoning":0,"cache":{"read":100,"write":0}},"title":"The command printed fixture.","agent":"build","model":{"id":"fake-model","providerID":"fake","variant":"default"},"version":"1.18.33","time":{"created":1790768250632,"updated":1790768251721},"permission":[{"permission":"question","pattern":"*","action":"deny"},{"permission":"plan_enter","pattern":"*","action":"deny"},{"permission":"plan_exit","pattern":"*","action":"deny"}]}}');
CREATE TABLE `permission` (
          `id` text PRIMARY KEY,
          `project_id` text NOT NULL,
          `action` text NOT NULL,
          `resource` text NOT NULL,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL,
          CONSTRAINT `fk_permission_project_id_project_id_fk` FOREIGN KEY (`project_id`) REFERENCES `project`(`id`) ON DELETE CASCADE
        );
CREATE TABLE `project_directory` (
          `project_id` text NOT NULL,
          `directory` text NOT NULL,
          `type` text,
          `strategy` text,
          `time_created` integer NOT NULL,
          CONSTRAINT `project_directory_pk` PRIMARY KEY(`project_id`, `directory`),
          CONSTRAINT `fk_project_directory_project_id_project_id_fk` FOREIGN KEY (`project_id`) REFERENCES `project`(`id`) ON DELETE CASCADE
        );
CREATE TABLE `project` (
          `id` text PRIMARY KEY,
          `worktree` text NOT NULL,
          `vcs` text,
          `name` text,
          `icon_url` text,
          `icon_url_override` text,
          `icon_color` text,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL,
          `time_initialized` integer,
          `sandboxes` text NOT NULL,
          `commands` text
        );
INSERT INTO project VALUES('global','/',NULL,NULL,NULL,NULL,NULL,1790768250340,1790768250340,NULL,'[]',NULL);
CREATE TABLE `message` (
          `id` text PRIMARY KEY,
          `session_id` text NOT NULL,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL,
          `data` text NOT NULL,
          CONSTRAINT `fk_message_session_id_session_id_fk` FOREIGN KEY (`session_id`) REFERENCES `session`(`id`) ON DELETE CASCADE
        );
INSERT INTO message VALUES('msg_0f21aff2e001y84CAhBLtC5Dm9','ses_f0de500f8ffeXvsiYJDOg5JtrU',1790768250670,1790768251687,'{"role":"user","time":{"created":1790768250670},"agent":"build","model":{"providerID":"fake","modelID":"fake-model"},"summary":{"diffs":[]}}');
INSERT INTO message VALUES('msg_0f21b0104001XDVLEwtN0JiIZk','ses_f0de500f8ffeXvsiYJDOg5JtrU',1790768251140,1790768251673,'{"parentID":"msg_0f21aff2e001y84CAhBLtC5Dm9","role":"assistant","mode":"build","agent":"build","path":{"cwd":"/work/proj","root":"/"},"cost":0,"tokens":{"total":128,"input":120,"output":8,"reasoning":0,"cache":{"write":0,"read":0}},"modelID":"fake-model","providerID":"fake","time":{"created":1790768251140,"completed":1790768251673},"finish":"tool-calls"}');
INSERT INTO message VALUES('msg_0f21b031b0014ZxYbhc4DxjZrb','ses_f0de500f8ffeXvsiYJDOg5JtrU',1790768251675,1790768251709,'{"parentID":"msg_0f21aff2e001y84CAhBLtC5Dm9","role":"assistant","mode":"build","agent":"build","path":{"cwd":"/work/proj","root":"/"},"cost":0,"tokens":{"total":156,"input":50,"output":6,"reasoning":0,"cache":{"write":0,"read":100}},"modelID":"fake-model","providerID":"fake","time":{"created":1790768251675,"completed":1790768251709},"finish":"stop"}');
CREATE TABLE `part` (
          `id` text PRIMARY KEY,
          `message_id` text NOT NULL,
          `session_id` text NOT NULL,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL,
          `data` text NOT NULL,
          CONSTRAINT `fk_part_message_id_message_id_fk` FOREIGN KEY (`message_id`) REFERENCES `message`(`id`) ON DELETE CASCADE
        );
INSERT INTO part VALUES('prt_0f21aff33001a2Ikz6KYrhKnir','msg_0f21aff2e001y84CAhBLtC5Dm9','ses_f0de500f8ffeXvsiYJDOg5JtrU',1790768251129,1790768251131,'{"type":"text","text":"\"run echo fixture please\""}');
INSERT INTO part VALUES('prt_0f21b02d3001tOYcQAJkloLqkx','msg_0f21b0104001XDVLEwtN0JiIZk','ses_f0de500f8ffeXvsiYJDOg5JtrU',1790768251603,1790768251604,'{"type":"step-start"}');
INSERT INTO part VALUES('prt_0f21b02d5001NcGUD0hmNfrPyr','msg_0f21b0104001XDVLEwtN0JiIZk','ses_f0de500f8ffeXvsiYJDOg5JtrU',1790768251605,1790768251667,'{"type":"tool","tool":"bash","callID":"call_1","state":{"status":"completed","input":{"command":"echo fixture"},"output":"fixture\n","metadata":{"output":"fixture\n","exit":0,"truncated":false},"title":"echo fixture","time":{"start":1790768251611,"end":1790768251666}}}');
INSERT INTO part VALUES('prt_0f21b0315001UbHx9JXmviMyiM','msg_0f21b0104001XDVLEwtN0JiIZk','ses_f0de500f8ffeXvsiYJDOg5JtrU',1790768251669,1790768251670,'{"reason":"tool-calls","type":"step-finish","tokens":{"total":128,"input":120,"output":8,"reasoning":0,"cache":{"write":0,"read":0}},"cost":0}');
INSERT INTO part VALUES('prt_0f21b03360013A63H7yUuGZnIK','msg_0f21b031b0014ZxYbhc4DxjZrb','ses_f0de500f8ffeXvsiYJDOg5JtrU',1790768251702,1790768251702,'{"type":"step-start"}');
INSERT INTO part VALUES('prt_0f21b0337001GtX6vvuhLhelY7','msg_0f21b031b0014ZxYbhc4DxjZrb','ses_f0de500f8ffeXvsiYJDOg5JtrU',1790768251703,1790768251705,'{"type":"text","text":"The command printed fixture.","time":{"start":1790768251703,"end":1790768251705}}');
INSERT INTO part VALUES('prt_0f21b033a001VTlTYA9mpcWy8P','msg_0f21b031b0014ZxYbhc4DxjZrb','ses_f0de500f8ffeXvsiYJDOg5JtrU',1790768251706,1790768251707,'{"reason":"stop","type":"step-finish","tokens":{"total":156,"input":50,"output":6,"reasoning":0,"cache":{"write":0,"read":100}},"cost":0}');
CREATE TABLE `session_context_epoch` (
          `session_id` text PRIMARY KEY,
          `baseline` text NOT NULL,
          `snapshot` text NOT NULL,
          `baseline_seq` integer NOT NULL,
          CONSTRAINT `fk_session_context_epoch_session_id_session_id_fk` FOREIGN KEY (`session_id`) REFERENCES `session`(`id`) ON DELETE CASCADE
        );
CREATE TABLE `session_input` (
          `id` text PRIMARY KEY,
          `session_id` text NOT NULL,
          `prompt` text NOT NULL,
          `delivery` text NOT NULL,
          `admitted_seq` integer NOT NULL,
          `promoted_seq` integer,
          `time_created` integer NOT NULL,
          CONSTRAINT `fk_session_input_session_id_session_id_fk` FOREIGN KEY (`session_id`) REFERENCES `session`(`id`) ON DELETE CASCADE
        );
CREATE TABLE `session_message` (
          `id` text PRIMARY KEY,
          `session_id` text NOT NULL,
          `type` text NOT NULL,
          `seq` integer NOT NULL,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL,
          `data` text NOT NULL,
          CONSTRAINT `fk_session_message_session_id_session_id_fk` FOREIGN KEY (`session_id`) REFERENCES `session`(`id`) ON DELETE CASCADE
        );
CREATE TABLE `session` (
          `id` text PRIMARY KEY,
          `project_id` text NOT NULL,
          `workspace_id` text,
          `parent_id` text,
          `slug` text NOT NULL,
          `directory` text NOT NULL,
          `path` text,
          `title` text NOT NULL,
          `version` text NOT NULL,
          `share_url` text,
          `summary_additions` integer,
          `summary_deletions` integer,
          `summary_files` integer,
          `summary_diffs` text,
          `metadata` text,
          `cost` real DEFAULT 0 NOT NULL,
          `tokens_input` integer DEFAULT 0 NOT NULL,
          `tokens_output` integer DEFAULT 0 NOT NULL,
          `tokens_reasoning` integer DEFAULT 0 NOT NULL,
          `tokens_cache_read` integer DEFAULT 0 NOT NULL,
          `tokens_cache_write` integer DEFAULT 0 NOT NULL,
          `revert` text,
          `permission` text,
          `agent` text,
          `model` text,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL,
          `time_compacting` integer,
          `time_archived` integer,
          CONSTRAINT `fk_session_project_id_project_id_fk` FOREIGN KEY (`project_id`) REFERENCES `project`(`id`) ON DELETE CASCADE
        );
INSERT INTO session VALUES('ses_f0de500f8ffeXvsiYJDOg5JtrU','global',NULL,NULL,'sunny-nebula','/work/proj','tmp/lazyagents-fixture-1634861929/work/proj','The command printed fixture.','1.18.33',NULL,0,0,0,NULL,NULL,0.0,170,14,0,100,0,NULL,'[{"permission":"question","pattern":"*","action":"deny"},{"permission":"plan_enter","pattern":"*","action":"deny"},{"permission":"plan_exit","pattern":"*","action":"deny"}]','build','{"id":"fake-model","providerID":"fake","variant":"default"}',1790768250632,1790768251721,NULL,NULL);
CREATE TABLE `todo` (
          `session_id` text NOT NULL,
          `content` text NOT NULL,
          `status` text NOT NULL,
          `priority` text NOT NULL,
          `position` integer NOT NULL,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL,
          CONSTRAINT `todo_pk` PRIMARY KEY(`session_id`, `position`),
          CONSTRAINT `fk_todo_session_id_session_id_fk` FOREIGN KEY (`session_id`) REFERENCES `session`(`id`) ON DELETE CASCADE
        );
CREATE TABLE `session_share` (
          `session_id` text PRIMARY KEY,
          `id` text NOT NULL,
          `secret` text NOT NULL,
          `url` text NOT NULL,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL,
          CONSTRAINT `fk_session_share_session_id_session_id_fk` FOREIGN KEY (`session_id`) REFERENCES `session`(`id`) ON DELETE CASCADE
        );
CREATE TABLE IF NOT EXISTS "migration" (id TEXT PRIMARY KEY, time_completed INTEGER NOT NULL);
INSERT INTO migration VALUES('20260127222353_familiar_lady_ursula',1790768250177);
INSERT INTO migration VALUES('20260211171708_add_project_commands',1790768250177);
INSERT INTO migration VALUES('20260213144116_wakeful_the_professor',1790768250177);
INSERT INTO migration VALUES('20260225215848_workspace',1790768250177);
INSERT INTO migration VALUES('20260227213759_add_session_workspace_id',1790768250177);
INSERT INTO migration VALUES('20260228203230_blue_harpoon',1790768250177);
INSERT INTO migration VALUES('20260303231226_add_workspace_fields',1790768250177);
INSERT INTO migration VALUES('20260309230000_move_org_to_state',1790768250177);
INSERT INTO migration VALUES('20260312043431_session_message_cursor',1790768250177);
INSERT INTO migration VALUES('20260323234822_events',1790768250177);
INSERT INTO migration VALUES('20260410174513_workspace-name',1790768250177);
INSERT INTO migration VALUES('20260413175956_chief_energizer',1790768250177);
INSERT INTO migration VALUES('20260423070820_add_icon_url_override',1790768250178);
INSERT INTO migration VALUES('20260427172553_slow_nightmare',1790768250178);
INSERT INTO migration VALUES('20260428004200_add_session_path',1790768250178);
INSERT INTO migration VALUES('20260501142318_next_venus',1790768250178);
INSERT INTO migration VALUES('20260504145000_add_sync_owner',1790768250178);
INSERT INTO migration VALUES('20260507164347_add_workspace_time',1790768250178);
INSERT INTO migration VALUES('20260510033149_session_usage',1790768250178);
INSERT INTO migration VALUES('20260511000411_data_migration_state',1790768250178);
INSERT INTO migration VALUES('20260511173437_session-metadata',1790768250178);
INSERT INTO migration VALUES('20260601010001_normalize_storage_paths',1790768250178);
INSERT INTO migration VALUES('20260601202201_amazing_prowler',1790768250179);
INSERT INTO migration VALUES('20260602002951_lowly_union_jack',1790768250179);
INSERT INTO migration VALUES('20260602182828_add_project_directories',1790768250179);
INSERT INTO migration VALUES('20260603001617_session_message_projection_indexes',1790768250179);
INSERT INTO migration VALUES('20260603040000_session_message_projection_order',1790768250179);
INSERT INTO migration VALUES('20260603141458_session_input_inbox',1790768250179);
INSERT INTO migration VALUES('20260603160727_jittery_ezekiel_stane',1790768250179);
INSERT INTO migration VALUES('20260604172448_event_sourced_session_input',1790768250179);
INSERT INTO migration VALUES('20260605003541_add_session_context_snapshot',1790768250179);
INSERT INTO migration VALUES('20260605042240_add_context_epoch_agent',1790768250179);
INSERT INTO migration VALUES('20260611035744_credential',1790768250179);
INSERT INTO migration VALUES('20260611192811_lush_chimera',1790768250179);
INSERT INTO migration VALUES('20260612174303_project_dir_strategy',1790768250179);
INSERT INTO migration VALUES('20260622142730_simplify_session_context_epoch',1790768250180);
INSERT INTO migration VALUES('20260622170816_reset_v2_session_state',1790768250180);
INSERT INTO migration VALUES('20260622202450_simplify_session_input',1790768250180);
CREATE UNIQUE INDEX `event_aggregate_seq_idx` ON `event` (`aggregate_id`,`seq`);
CREATE INDEX `event_aggregate_type_seq_idx` ON `event` (`aggregate_id`,`type`,`seq`);
CREATE UNIQUE INDEX `permission_project_action_resource_idx` ON `permission` (`project_id`,`action`,`resource`);
CREATE INDEX `message_session_time_created_id_idx` ON `message` (`session_id`,`time_created`,`id`);
CREATE INDEX `part_message_id_id_idx` ON `part` (`message_id`,`id`);
CREATE INDEX `part_session_idx` ON `part` (`session_id`);
CREATE INDEX `session_input_session_pending_delivery_seq_idx` ON `session_input` (`session_id`,`promoted_seq`,`delivery`,`admitted_seq`);
CREATE UNIQUE INDEX `session_input_session_admitted_seq_idx` ON `session_input` (`session_id`,`admitted_seq`);
CREATE UNIQUE INDEX `session_input_session_promoted_seq_idx` ON `session_input` (`session_id`,`promoted_seq`);
CREATE UNIQUE INDEX `session_message_session_seq_idx` ON `session_message` (`session_id`,`seq`);
CREATE INDEX `session_message_session_type_seq_idx` ON `session_message` (`session_id`,`type`,`seq`);
CREATE INDEX `session_message_session_time_created_id_idx` ON `session_message` (`session_id`,`time_created`,`id`);
CREATE INDEX `session_message_time_created_idx` ON `session_message` (`time_created`);
CREATE INDEX `session_project_idx` ON `session` (`project_id`);
CREATE INDEX `session_workspace_idx` ON `session` (`workspace_id`);
CREATE INDEX `session_parent_idx` ON `session` (`parent_id`);
CREATE INDEX `todo_session_idx` ON `todo` (`session_id`);
COMMIT;
