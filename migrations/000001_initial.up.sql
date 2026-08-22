CREATE DOMAIN entity_version AS bigint CHECK (VALUE > 0);
CREATE TYPE alarm_lifecycle AS ENUM ('open', 'acknowledged', 'recovering', 'resolved', 'manual_takeover');

CREATE TABLE domain_objects (
    kind text NOT NULL,
    id text NOT NULL,
    site_id text,
    secondary_key text,
    version entity_version NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (kind, id)
);
CREATE UNIQUE INDEX uq_domain_secondary ON domain_objects(kind, site_id, secondary_key) WHERE secondary_key IS NOT NULL;
CREATE INDEX ix_domain_site_kind ON domain_objects(site_id, kind, id);

CREATE TABLE telemetry_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id text NOT NULL,
    crane_id text NOT NULL,
    sensor_id text NOT NULL,
    sequence bigint NOT NULL CHECK (sequence >= 0),
    observed_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL,
    quality text NOT NULL,
    evidence_checksum text NOT NULL,
    quarantined boolean NOT NULL DEFAULT false,
    quarantine_reason text,
    payload jsonb NOT NULL,
    UNIQUE (crane_id, event_id)
);
CREATE INDEX ix_telemetry_crane_time ON telemetry_events(crane_id, observed_at DESC);
CREATE INDEX ix_telemetry_sensor_sequence ON telemetry_events(sensor_id, sequence DESC);

CREATE TABLE safety_decisions (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    crane_id text NOT NULL,
    level text NOT NULL,
    interlock boolean NOT NULL,
    evaluated_at timestamptz NOT NULL,
    payload jsonb NOT NULL
);
CREATE INDEX ix_decision_crane_time ON safety_decisions(crane_id, evaluated_at DESC);

CREATE TABLE safety_clearance (
    crane_id text PRIMARY KEY,
    clear_since timestamptz NOT NULL
);

CREATE TABLE alarms (
    id text PRIMARY KEY,
    site_id text NOT NULL,
    crane_id text NOT NULL,
    telemetry_event_id text NOT NULL,
    status alarm_lifecycle NOT NULL,
    level text NOT NULL,
    interlock boolean NOT NULL,
    version entity_version NOT NULL,
    evidence_hash text NOT NULL,
    opened_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    payload jsonb NOT NULL,
    UNIQUE (crane_id, telemetry_event_id)
);
CREATE INDEX ix_alarms_site_status ON alarms(site_id, status, opened_at DESC);

CREATE TABLE audit_records (
    sequence bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id text NOT NULL UNIQUE,
    site_id text NOT NULL,
    actor_id text NOT NULL,
    source text NOT NULL,
    action text NOT NULL,
    resource text NOT NULL,
    resource_id text NOT NULL,
    before_data jsonb,
    after_data jsonb,
    reason text NOT NULL,
    request_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    previous_hash text,
    record_hash text NOT NULL UNIQUE
);
CREATE INDEX ix_audit_site_time ON audit_records(site_id, occurred_at, sequence);

CREATE TABLE fleet_relations (
    site_id text NOT NULL,
    crane_a_id text NOT NULL,
    crane_b_id text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    version entity_version NOT NULL,
    payload jsonb NOT NULL,
    PRIMARY KEY(site_id, crane_a_id, crane_b_id),
    CHECK (crane_a_id < crane_b_id)
);

CREATE TABLE crane_poses (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    crane_id text NOT NULL,
    observed_at timestamptz NOT NULL,
    payload jsonb NOT NULL,
    UNIQUE(crane_id, observed_at)
);

CREATE TABLE collision_risks (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    crane_a_id text NOT NULL,
    crane_b_id text NOT NULL,
    predicted_at timestamptz NOT NULL,
    critical boolean NOT NULL,
    payload jsonb NOT NULL
);

CREATE TABLE crane_stop_records (
    id text PRIMARY KEY,
    crane_id text NOT NULL,
    stopped_at timestamptz NOT NULL,
    resumed_at timestamptz,
    payload jsonb NOT NULL
);
CREATE UNIQUE INDEX uq_crane_open_stop ON crane_stop_records(crane_id) WHERE resumed_at IS NULL;

CREATE TABLE refresh_tokens (
    id text PRIMARY KEY,
    user_id text NOT NULL,
    digest text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    version entity_version NOT NULL,
    payload jsonb NOT NULL
);

CREATE TABLE regulatory_reports (
    id text PRIMARY KEY,
    site_id text NOT NULL,
    generated_at timestamptz NOT NULL,
    content_hash text NOT NULL UNIQUE,
    retention_end timestamptz NOT NULL CHECK (retention_end > generated_at),
    payload jsonb NOT NULL
);
CREATE INDEX ix_reports_site_time ON regulatory_reports(site_id, generated_at DESC, id DESC);

CREATE TABLE outbox_messages (
    id text PRIMARY KEY,
    topic text NOT NULL,
    message_key text NOT NULL,
    payload jsonb NOT NULL,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts integer NOT NULL CHECK (max_attempts > 0),
    available_at timestamptz NOT NULL,
    delivered_at timestamptz,
    dead_at timestamptz,
    last_error text
);
CREATE INDEX ix_outbox_due ON outbox_messages(available_at) WHERE delivered_at IS NULL AND dead_at IS NULL;

CREATE TABLE file_metadata (
    id text PRIMARY KEY,
    site_id text NOT NULL,
    name text NOT NULL,
    mime text NOT NULL,
    size_bytes bigint NOT NULL CHECK (size_bytes > 0),
	checksum text NOT NULL,
    storage_path text NOT NULL UNIQUE,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_files_site ON file_metadata(site_id, created_at DESC);
CREATE UNIQUE INDEX uq_files_site_checksum ON file_metadata(site_id, checksum);
