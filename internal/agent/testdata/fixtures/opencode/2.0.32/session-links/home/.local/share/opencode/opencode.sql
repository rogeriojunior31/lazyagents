PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
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
INSERT INTO event_sequence VALUES('ses_eb032912bffeZp5Ymu2wiLr0Z2',35,NULL);
INSERT INTO event_sequence VALUES('ses_eb0328faeffekX4vw5ZLUFnWaU',17,NULL);
CREATE TABLE `event` (
          `id` text PRIMARY KEY,
          `aggregate_id` text NOT NULL,
          `seq` integer NOT NULL,
          `created` integer DEFAULT 0 NOT NULL,
          `type` text NOT NULL,
          `data` text NOT NULL,
          CONSTRAINT `fk_event_aggregate_id_event_sequence_aggregate_id_fk` FOREIGN KEY (`aggregate_id`) REFERENCES `event_sequence`(`aggregate_id`) ON DELETE CASCADE
        );
CREATE TABLE `kv` (
          `key` text PRIMARY KEY,
          `value` text NOT NULL,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL
        );
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
          `time_active` integer DEFAULT 0 NOT NULL,
          `time_archived` integer,
          `sandboxes` text NOT NULL,
          `commands` text
        );
INSERT INTO project VALUES('d5039b55345b66b69cd0ed27f0c1f9fd833c99c5','/work/proj',NULL,NULL,NULL,NULL,NULL,1792340225700,1792340225700,NULL,1792340226630,NULL,'[]',NULL);
CREATE TABLE `instruction_blob` (
          `hash` text PRIMARY KEY,
          `value` text
        );
INSERT INTO instruction_blob VALUES('e6428fd7742161f6c3a6c86e53cbbe2c6e4dcc462bbf1a1a07bc4fcbeb9e0b0b','{"total":8,"shown":8,"namespaces":[{"name":"opencode","description":"Tools for managing OpenCode itself, such as working with sessions and worktrees, searching the available models, and reading MCP resources.","count":8,"entries":[{"path":"opencode.list_mcp_resources","line":"  - tools.opencode.list_mcp_resources({\n  /** MCP server name as configured. Omit to list every server. */\n  server?: string,\n}): Promise<{\n  resources: Array<{\n      server: string,\n      name: string,\n      uri: string,\n      description?: string,\n      mimeType?: string,\n    }>,\n  templates: Array<{\n      server: string,\n      name: string,\n      uriTemplate: string,\n      description?: string,\n      mimeType?: string,\n    }>,\n}> // List documents, records, and other data exposed by MCP servers. Use this when the user refers to something that is no..."},{"path":"opencode.models","line":"  - tools.opencode.models({\n  /** Text to search for in model names and IDs. */\n  query?: string,\n  /** Provider ID or name to filter by. Try your own provider first. */\n  provider?: string,\n  /** Include older versions of each model family. By default only the newest version is listed. */\n  all?: boolean,\n  /** Maximum number of models to return. Defaults to 20. @integer @minimum 1 @maximum 100 */\n  limit?: number,\n  /** Number of models to skip, for paging through results. @integer @minimum 0 */\n  offset?: number,\n}): Promise<{\n  /** Matching models grouped by provider. Your own provider comes first. */\n  providers: Array<{\n      id: string,\n      name: string,\n      /** Newest first. */\n      models: Array<{\n          /** providerID/modelID */\n          id: string,\n          name: string,\n          /** Release date as a Unix timestamp in milliseconds, or 0 when unknown. */\n          released: number,\n          variants: Array<string>,\n          /** Pricing in USD per million tokens. */\n          cost: Array<{\n              tier?: {\n                type: \"context\",\n                /** @integer */\n                size: number,\n              },\n              input: number,\n              output: number,\n              cache: {\n                read: number,\n                write: number,\n              },\n            }>,\n          status: \"alpha\" | \"beta\" | \"deprecated\" | \"active\",\n        }>,\n    }>,\n  /** Number of matching models across all pages. @integer */\n  total: number,\n  /** Offset of the next page, or null on the last page. */\n  next: number | null,\n}> // Search the models available to use. Use this to turn a model name the user mentions into an exact reference before ru..."},{"path":"opencode.read_mcp_resource","line":"  - tools.opencode.read_mcp_resource({\n  /** The server field of the discovered resource. */\n  server: string,\n  /** Exact resource URI from discovery, an expanded uriTemplate, or the user. */\n  uri: string,\n}): Promise<{\n  server: string,\n  uri: string,\n  contents: Array<{\n        type: \"text\",\n        uri: string,\n        text: string,\n        mimeType?: string,\n      } | {\n        type: \"blob\",\n        uri: string,\n        blob: string,\n        mimeType?: string,\n      }>,\n}> // Read one MCP resource by server and URI. Not for local files. Always return the full contents rather than slicing or ..."},{"path":"opencode.session_move","line":"  - tools.opencode.session_move({\n  /** Omit to move the current session. @pattern ^ses */\n  sessionID?: string,\n  /** Destination directory, relative to the target session''s directory or absolute. Supports ~. @minLength 1 */\n  directory: string,\n}): Promise<{\n  /** @pattern ^ses */\n  sessionID: string,\n  directory: string,\n}> // Move a session to another directory, or omit sessionID to move the current session. The current session moves at the ..."},{"path":"opencode.session_rename","line":"  - tools.opencode.session_rename({\n  /** Omit to rename the current session. @pattern ^ses */\n  sessionID?: string,\n  /** New session title. @minLength 1 */\n  title: string,\n}): Promise<{\n  /** @pattern ^ses */\n  sessionID: string,\n  title: string,\n}> // Rename a session, or omit sessionID to rename the current session. Use a short, specific title that summarizes the wo..."},{"path":"opencode.worktree_create","line":"  - tools.opencode.worktree_create({\n  name: string,\n}): Promise<{\n  directory: string,\n}> // Create a named worktree. Returns its directory."},{"path":"opencode.worktree_list","line":"  - tools.opencode.worktree_list(): Promise<{\n  worktrees: Array<{\n      directory: string,\n      strategy?: string,\n    }>,\n}> // List a repository''s worktrees."},{"path":"opencode.worktree_remove","line":"  - tools.opencode.worktree_remove({\n  /** Absolute path of the worktree to remove. @minLength 1 */\n  directory: string,\n  /** Discard modified and untracked files. Only set this when the user has confirmed forced removal. */\n  force?: boolean,\n}): Promise<{\n  directory: string,\n  removed: boolean,\n}> // Remove a worktree. The repository location selects configuration; directory identifies the worktree to remove."}]}]}');
INSERT INTO instruction_blob VALUES('8394362d28c82fa4d13c7a9f5568c05680f6687e07db8f426a0e3c92f3027dc9','[{"id":"opencode","name":"OpenCode","description":"Use this skill for any question about OpenCode itself, including how OpenCode works, using or configuring it, migrating from V1 to V2, troubleshooting it, developing plugins or integrations, using the OpenCode SDK, clients, server, or API, and contributing to the OpenCode codebase. Also use it for OpenCode agents, commands, skills, tools, permissions, MCP servers, providers, models, themes, keybinds, formatters, the CLI, TUI, desktop app, and web app."},{"id":"report","name":"Report","description":"Use when the user wants to report an opencode issue or bug. Collect standard diagnostics, add user-specific reproduction context, and publish the issue with GitHub CLI."}]');
INSERT INTO instruction_blob VALUES('758845208f3ecf803dbb35570ace65e10bf51fa67715d5f28465295ee0c42291','"Sun Oct 18 2026"');
INSERT INTO instruction_blob VALUES('b75b6eb6f48c7a3a7d31d515d958dae500a65b04977cf244552c906360224f7b','"<env>\n  Working directory: /work/proj\n  Workspace root folder: /work/proj\n  Is directory a git repo: no\n  Platform: linux\n  Prefer /tmp/rec/opencode over generic system temporary directories such as /tmp; it is pre-created and approved for external access.\n</env>"');
CREATE TABLE `instruction_entry` (
          `session_id` text NOT NULL,
          `key` text NOT NULL,
          `value` text,
          `removed` integer DEFAULT false NOT NULL,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL,
          CONSTRAINT `instruction_entry_pk` PRIMARY KEY(`session_id`, `key`),
          CONSTRAINT `fk_instruction_entry_session_id_session_v2_id_fk` FOREIGN KEY (`session_id`) REFERENCES `session_v2`(`id`) ON DELETE CASCADE
        );
CREATE TABLE `instruction_state` (
          `session_id` text PRIMARY KEY,
          `epoch_start` integer NOT NULL,
          `through_seq` integer NOT NULL,
          `initial_values` text NOT NULL,
          `current_values` text NOT NULL,
          CONSTRAINT `fk_instruction_state_session_id_session_v2_id_fk` FOREIGN KEY (`session_id`) REFERENCES `session_v2`(`id`) ON DELETE CASCADE
        );
INSERT INTO instruction_state VALUES('ses_eb032912bffeZp5Ymu2wiLr0Z2',3,3,'{"core/codemode":"e6428fd7742161f6c3a6c86e53cbbe2c6e4dcc462bbf1a1a07bc4fcbeb9e0b0b","core/skill-guidance":"8394362d28c82fa4d13c7a9f5568c05680f6687e07db8f426a0e3c92f3027dc9","core/date":"758845208f3ecf803dbb35570ace65e10bf51fa67715d5f28465295ee0c42291","core/environment":"b75b6eb6f48c7a3a7d31d515d958dae500a65b04977cf244552c906360224f7b"}','{"core/codemode":"e6428fd7742161f6c3a6c86e53cbbe2c6e4dcc462bbf1a1a07bc4fcbeb9e0b0b","core/skill-guidance":"8394362d28c82fa4d13c7a9f5568c05680f6687e07db8f426a0e3c92f3027dc9","core/date":"758845208f3ecf803dbb35570ace65e10bf51fa67715d5f28465295ee0c42291","core/environment":"b75b6eb6f48c7a3a7d31d515d958dae500a65b04977cf244552c906360224f7b"}');
INSERT INTO instruction_state VALUES('ses_eb0328faeffekX4vw5ZLUFnWaU',3,3,'{"core/date":"758845208f3ecf803dbb35570ace65e10bf51fa67715d5f28465295ee0c42291","core/environment":"b75b6eb6f48c7a3a7d31d515d958dae500a65b04977cf244552c906360224f7b"}','{"core/date":"758845208f3ecf803dbb35570ace65e10bf51fa67715d5f28465295ee0c42291","core/environment":"b75b6eb6f48c7a3a7d31d515d958dae500a65b04977cf244552c906360224f7b"}');
CREATE TABLE `session_inbox` (
          `id` text PRIMARY KEY,
          `session_id` text NOT NULL,
          `type` text NOT NULL,
          `payload` text NOT NULL,
          `delivery` text NOT NULL,
          `enqueued_seq` integer NOT NULL,
          `time_created` integer NOT NULL,
          CONSTRAINT `fk_session_inbox_session_id_session_v2_id_fk` FOREIGN KEY (`session_id`) REFERENCES `session_v2`(`id`) ON DELETE CASCADE
        );
CREATE TABLE `session_message` (
          `id` text PRIMARY KEY,
          `session_id` text NOT NULL,
          `type` text NOT NULL,
          `seq` integer NOT NULL,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL,
          `data` text NOT NULL,
          CONSTRAINT `fk_session_message_session_id_session_v2_id_fk` FOREIGN KEY (`session_id`) REFERENCES `session_v2`(`id`) ON DELETE CASCADE
        );
INSERT INTO session_message VALUES('msg_14fcd6fd5001U393cyK8SrLka6','ses_eb032912bffeZp5Ymu2wiLr0Z2','user',4,1792340226073,1792340226074,'{"time":{"created":1792340226073},"text":"\"delegate fixture please\"","files":[]}');
INSERT INTO session_message VALUES('msg_14fcd7029001aNQlvTtiSGegXt','ses_eb032912bffeZp5Ymu2wiLr0Z2','assistant',5,1792340226100,1792340226244,'{"time":{"created":1792340226100,"streamed":1792340226120,"completed":1792340226244},"agent":"build","model":{"id":"fake-model","providerID":"fake"},"content":[{"type":"tool","id":"call_1","name":"subagent","executed":false,"state":{"status":"completed","input":{"agent":"explore","description":"Print fixture","prompt":"run echo fixture please"},"content":[{"type":"text","text":"<subagent sessionID=\"ses_eb0328faeffekX4vw5ZLUFnWaU\" state=\"completed\">\nThe command printed fixture.\n</subagent>"}],"metadata":{"sessionID":"ses_eb0328faeffekX4vw5ZLUFnWaU","status":"completed","truncated":false}},"time":{"created":1792340226114,"ran":1792340226119,"completed":1792340226242}}],"finish":"tool-calls","rawFinish":"tool_calls","cost":0,"tokens":{"input":120,"output":8,"reasoning":0,"cache":{"read":0,"write":0}}}');
INSERT INTO session_message VALUES('msg_14fcd7055001gODV3XQwZKrf7O','ses_eb0328faeffekX4vw5ZLUFnWaU','user',4,1792340226143,1792340226143,'{"time":{"created":1792340226143},"text":"You are a subagent spawned by another session.\nrun echo fixture please"}');
INSERT INTO session_message VALUES('msg_14fcd7063001zvIv440KPG3OWl','ses_eb0328faeffekX4vw5ZLUFnWaU','assistant',5,1792340226154,1792340226210,'{"time":{"created":1792340226154,"streamed":1792340226163,"completed":1792340226209},"agent":"explore","model":{"id":"fake-model","providerID":"fake"},"content":[{"type":"tool","id":"call_1","name":"shell","executed":false,"state":{"status":"completed","input":{"command":"echo fixture"},"content":[{"type":"text","text":"fixture\n"}],"metadata":{"status":"completed","truncated":false,"exit":0}},"time":{"created":1792340226159,"ran":1792340226162,"completed":1792340226207}}],"finish":"tool-calls","rawFinish":"tool_calls","cost":0,"tokens":{"input":120,"output":8,"reasoning":0,"cache":{"read":0,"write":0}}}');
INSERT INTO session_message VALUES('msg_14fcd70aa001kwFyNNtoFhAn4n','ses_eb0328faeffekX4vw5ZLUFnWaU','assistant',12,1792340226225,1792340226239,'{"time":{"created":1792340226225,"streamed":1792340226238,"completed":1792340226238},"agent":"explore","model":{"id":"fake-model","providerID":"fake"},"content":[{"type":"text","text":"The command printed fixture."}],"finish":"stop","rawFinish":"stop","cost":0,"tokens":{"input":50,"output":6,"reasoning":0,"cache":{"read":100,"write":0}}}');
INSERT INTO session_message VALUES('msg_14fcd70c0002xYTeo6BFpWVQOT','ses_eb0328faeffekX4vw5ZLUFnWaU','idle',17,1792340226240,1792340226241,'{"time":{"created":1792340226240},"outcome":"succeeded"}');
INSERT INTO session_message VALUES('msg_14fcd70cb001S31TEqIwn733CN','ses_eb032912bffeZp5Ymu2wiLr0Z2','assistant',14,1792340226258,1792340226266,'{"time":{"created":1792340226258,"streamed":1792340226265,"completed":1792340226265},"agent":"build","model":{"id":"fake-model","providerID":"fake"},"content":[{"type":"text","text":"The command printed fixture."}],"finish":"stop","rawFinish":"stop","cost":0,"tokens":{"input":50,"output":6,"reasoning":0,"cache":{"read":100,"write":0}}}');
INSERT INTO session_message VALUES('msg_14fcd70db001siXOTOhesQvrSl','ses_eb032912bffeZp5Ymu2wiLr0Z2','idle',19,1792340226267,1792340226267,'{"time":{"created":1792340226267},"outcome":"succeeded"}');
INSERT INTO session_message VALUES('msg_14fcd7315001dRerEhLOu8WPUk','ses_eb032912bffeZp5Ymu2wiLr0Z2','user',22,1792340226969,1792340226970,'{"time":{"created":1792340226969},"text":"\"resume fixture please\"","files":[]}');
INSERT INTO session_message VALUES('msg_14fcd73a10011STUC2QThHzdVX','ses_eb032912bffeZp5Ymu2wiLr0Z2','assistant',23,1792340226987,1792340227048,'{"time":{"created":1792340226987,"streamed":1792340227006,"completed":1792340227047},"agent":"build","model":{"id":"fake-model","providerID":"fake"},"content":[{"type":"tool","id":"call_1","name":"shell","executed":false,"state":{"status":"completed","input":{"command":"echo fixture"},"content":[{"type":"text","text":"fixture\n"}],"metadata":{"status":"completed","truncated":false,"exit":0}},"time":{"created":1792340226999,"ran":1792340227005,"completed":1792340227045}}],"finish":"tool-calls","rawFinish":"tool_calls","cost":0,"tokens":{"input":120,"output":8,"reasoning":0,"cache":{"read":0,"write":0}}}');
INSERT INTO session_message VALUES('msg_14fcd73f100140ng8kK0X6hmuW','ses_eb032912bffeZp5Ymu2wiLr0Z2','assistant',30,1792340227064,1792340227075,'{"time":{"created":1792340227064,"streamed":1792340227073,"completed":1792340227075},"agent":"build","model":{"id":"fake-model","providerID":"fake"},"content":[{"type":"text","text":"The command printed fixture."}],"finish":"stop","rawFinish":"stop","cost":0,"tokens":{"input":50,"output":6,"reasoning":0,"cache":{"read":100,"write":0}}}');
INSERT INTO session_message VALUES('msg_14fcd7405001TTkd63jNWJ7FMm','ses_eb032912bffeZp5Ymu2wiLr0Z2','idle',35,1792340227077,1792340227077,'{"time":{"created":1792340227077},"outcome":"succeeded"}');
CREATE TABLE `session_pending` (
          `id` text PRIMARY KEY,
          `session_id` text NOT NULL,
          `type` text NOT NULL,
          `data` text NOT NULL,
          `delivery` text,
          `admitted_seq` integer NOT NULL,
          `time_created` integer NOT NULL,
          CONSTRAINT `fk_session_pending_session_id_session_v2_id_fk` FOREIGN KEY (`session_id`) REFERENCES `session_v2`(`id`) ON DELETE CASCADE
        );
CREATE TABLE `session_v2` (
          `id` text PRIMARY KEY,
          `project_id` text NOT NULL,
          `workspace_id` text,
          `parent_id` text,
          `fork_session_id` text,
          `fork_boundary` text,
          `slug` text NOT NULL,
          `directory` text NOT NULL,
          `path` text,
          `title` text,
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
          `time_idle` integer,
          `time_viewed` integer,
          `idle_outcome` text,
          `time_compacting` integer,
          `time_archived` integer,
          `time_suspended` integer,
          `resume_attempts` integer DEFAULT 0 NOT NULL,
          CONSTRAINT `fk_session_v2_project_id_project_id_fk` FOREIGN KEY (`project_id`) REFERENCES `project`(`id`) ON DELETE CASCADE
        );
INSERT INTO session_v2 VALUES('ses_eb032912bffeZp5Ymu2wiLr0Z2','d5039b55345b66b69cd0ed27f0c1f9fd833c99c5',NULL,NULL,NULL,NULL,'mighty-canyon','/work/proj','','The command printed fixture.','2.0.32',NULL,NULL,NULL,NULL,NULL,NULL,0,390,34,0,300,0,NULL,NULL,NULL,NULL,1792340225987,1792340227075,1792340227077,NULL,'succeeded',NULL,NULL,NULL,0);
INSERT INTO session_v2 VALUES('ses_eb0328faeffekX4vw5ZLUFnWaU','d5039b55345b66b69cd0ed27f0c1f9fd833c99c5',NULL,'ses_eb032912bffeZp5Ymu2wiLr0Z2',NULL,NULL,'gentle-cabin','/work/proj','','Print fixture','2.0.32',NULL,NULL,NULL,NULL,NULL,NULL,0,170,14,0,100,0,NULL,NULL,'explore',NULL,1792340226131,1792340226238,1792340226240,NULL,'succeeded',NULL,NULL,NULL,0);
CREATE TABLE `workspace` (
          `id` text PRIMARY KEY,
          `provider` text NOT NULL,
          `binding` text,
          `created_at` integer NOT NULL,
          `last_used_at` integer NOT NULL
        );
CREATE TABLE `worktree` (
          `project_id` text NOT NULL,
          `directory` text NOT NULL,
          `strategy` text,
          `time_created` integer NOT NULL,
          CONSTRAINT `worktree_pk` PRIMARY KEY(`project_id`, `directory`),
          CONSTRAINT `fk_worktree_project_id_project_id_fk` FOREIGN KEY (`project_id`) REFERENCES `project`(`id`) ON DELETE CASCADE
        );
CREATE TABLE IF NOT EXISTS "migration" (id TEXT PRIMARY KEY, time_completed INTEGER NOT NULL);
INSERT INTO migration VALUES('20260127222353_familiar_lady_ursula',1792340225667);
INSERT INTO migration VALUES('20260211171708_add_project_commands',1792340225667);
INSERT INTO migration VALUES('20260213144116_wakeful_the_professor',1792340225667);
INSERT INTO migration VALUES('20260225215848_workspace',1792340225667);
INSERT INTO migration VALUES('20260227213759_add_session_workspace_id',1792340225667);
INSERT INTO migration VALUES('20260228203230_blue_harpoon',1792340225667);
INSERT INTO migration VALUES('20260303231226_add_workspace_fields',1792340225667);
INSERT INTO migration VALUES('20260309230000_move_org_to_state',1792340225667);
INSERT INTO migration VALUES('20260312043431_session_message_cursor',1792340225667);
INSERT INTO migration VALUES('20260323234822_events',1792340225667);
INSERT INTO migration VALUES('20260410174513_workspace-name',1792340225668);
INSERT INTO migration VALUES('20260413175956_chief_energizer',1792340225668);
INSERT INTO migration VALUES('20260423070820_add_icon_url_override',1792340225668);
INSERT INTO migration VALUES('20260427172553_slow_nightmare',1792340225668);
INSERT INTO migration VALUES('20260428004200_add_session_path',1792340225668);
INSERT INTO migration VALUES('20260501142318_next_venus',1792340225668);
INSERT INTO migration VALUES('20260504145000_add_sync_owner',1792340225668);
INSERT INTO migration VALUES('20260507164347_add_workspace_time',1792340225668);
INSERT INTO migration VALUES('20260510033149_session_usage',1792340225668);
INSERT INTO migration VALUES('20260511000411_data_migration_state',1792340225668);
INSERT INTO migration VALUES('20260511173437_session-metadata',1792340225668);
INSERT INTO migration VALUES('20260601010001_normalize_storage_paths',1792340225668);
INSERT INTO migration VALUES('20260601202201_amazing_prowler',1792340225668);
INSERT INTO migration VALUES('20260602002951_lowly_union_jack',1792340225668);
INSERT INTO migration VALUES('20260602182828_add_project_directories',1792340225668);
INSERT INTO migration VALUES('20260603001617_session_message_projection_indexes',1792340225668);
INSERT INTO migration VALUES('20260603040000_session_message_projection_order',1792340225668);
INSERT INTO migration VALUES('20260603141458_session_input_inbox',1792340225668);
INSERT INTO migration VALUES('20260603160727_jittery_ezekiel_stane',1792340225668);
INSERT INTO migration VALUES('20260604172448_event_sourced_session_input',1792340225669);
INSERT INTO migration VALUES('20260605003541_add_session_context_snapshot',1792340225669);
INSERT INTO migration VALUES('20260605042240_add_context_epoch_agent',1792340225669);
INSERT INTO migration VALUES('20260611035744_credential',1792340225669);
INSERT INTO migration VALUES('20260611192811_lush_chimera',1792340225669);
INSERT INTO migration VALUES('20260612174303_project_dir_strategy',1792340225669);
INSERT INTO migration VALUES('20260622142730_simplify_session_context_epoch',1792340225669);
INSERT INTO migration VALUES('20260622170816_reset_v2_session_state',1792340225669);
INSERT INTO migration VALUES('20260622202450_simplify_session_input',1792340225669);
INSERT INTO migration VALUES('20260804233008_loose_psylocke',1792340225669);
INSERT INTO migration VALUES('20260805200742_import_legacy_credentials',1792340225669);
INSERT INTO migration VALUES('20260808023530_workspace_domain',1792340225669);
INSERT INTO migration VALUES('20260811161259_execution_claim_attempts',1792340225669);
INSERT INTO migration VALUES('20260812181746_session_inbox',1792340225669);
INSERT INTO migration VALUES('20260812213948_worktree',1792340225669);
INSERT INTO migration VALUES('20260819222447_session_viewed_state',1792340225669);
INSERT INTO migration VALUES('20260823191254_nullable_workspace_binding',1792340225669);
INSERT INTO migration VALUES('20260910120000_clear_v1_session_permission',1792340225669);
INSERT INTO migration VALUES('20260923013825_project_time_active',1792340225669);
INSERT INTO migration VALUES('20261007190000_azure_cli_external_credential',1792340225669);
INSERT INTO migration VALUES('20261010180456_project_time_archived',1792340225670);
CREATE UNIQUE INDEX `event_aggregate_seq_idx` ON `event` (`aggregate_id`,`seq`);
CREATE INDEX `event_aggregate_type_seq_idx` ON `event` (`aggregate_id`,`type`,`seq`);
CREATE UNIQUE INDEX `permission_project_action_resource_idx` ON `permission` (`project_id`,`action`,`resource`);
CREATE INDEX `session_inbox_session_delivery_seq_idx` ON `session_inbox` (`session_id`,`delivery`,`enqueued_seq`);
CREATE UNIQUE INDEX `session_inbox_session_enqueued_seq_idx` ON `session_inbox` (`session_id`,`enqueued_seq`);
CREATE UNIQUE INDEX `session_message_session_seq_idx` ON `session_message` (`session_id`,`seq`);
CREATE INDEX `session_message_session_type_seq_idx` ON `session_message` (`session_id`,`type`,`seq`);
CREATE INDEX `session_message_session_time_created_id_idx` ON `session_message` (`session_id`,`time_created`,`id`);
CREATE INDEX `session_message_time_created_idx` ON `session_message` (`time_created`);
CREATE INDEX `session_pending_session_delivery_seq_idx` ON `session_pending` (`session_id`,`delivery`,`admitted_seq`);
CREATE UNIQUE INDEX `session_pending_session_compaction_idx` ON `session_pending` (`session_id`) WHERE "session_pending"."type" = 'compaction';
CREATE UNIQUE INDEX `session_pending_session_admitted_seq_idx` ON `session_pending` (`session_id`,`admitted_seq`);
CREATE INDEX `session_v2_project_idx` ON `session_v2` (`project_id`);
CREATE INDEX `session_v2_workspace_idx` ON `session_v2` (`workspace_id`);
CREATE INDEX `session_v2_parent_idx` ON `session_v2` (`parent_id`);
CREATE INDEX `session_v2_time_suspended_idx` ON `session_v2` (`time_suspended`) WHERE "session_v2"."time_suspended" is not null;
COMMIT;
