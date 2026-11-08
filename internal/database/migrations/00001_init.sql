-- +goose Up
CREATE TABLE media (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    preset text NOT NULL,
    storage_key text NOT NULL UNIQUE,
    width integer NOT NULL,
    height integer NOT NULL,
    bytes bigint NOT NULL,
    variants jsonb NOT NULL DEFAULT '[]',
    alt_text text NOT NULL DEFAULT '',
    is_private boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    phone text NOT NULL UNIQUE,
    name text NOT NULL,
    password_hash text NOT NULL,
    role text NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member')),
    level text NOT NULL DEFAULT 'beginner' CHECK (level IN ('beginner', 'intermediate', 'advanced')),
    refund_bank_name text NOT NULL DEFAULT '',
    refund_account_number text NOT NULL DEFAULT '',
    refund_account_name text NOT NULL DEFAULT '',
    is_active boolean NOT NULL DEFAULT true,
    must_change_password boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE auth_sessions (
    token_hash bytea PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    user_agent text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX auth_sessions_user_id_idx ON auth_sessions (user_id);

CREATE TABLE settings (
    key text PRIMARY KEY,
    value jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE venues (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name text NOT NULL,
    address text NOT NULL DEFAULT '',
    maps_url text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE session_templates (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name text NOT NULL,
    venue_id bigint NOT NULL REFERENCES venues (id) ON DELETE RESTRICT,
    weekday smallint NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    start_time time NOT NULL,
    end_time time NOT NULL,
    court_count integer NOT NULL CHECK (court_count > 0),
    capacity integer NOT NULL CHECK (capacity > 0),
    price bigint NOT NULL CHECK (price >= 0),
    point_rule text NOT NULL DEFAULT '',
    level_label text NOT NULL DEFAULT '',
    shuttlecock_included boolean NOT NULL DEFAULT true,
    unique_code_enabled boolean NOT NULL DEFAULT true,
    notes text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug text NOT NULL UNIQUE,
    title text NOT NULL,
    template_id bigint REFERENCES session_templates (id) ON DELETE SET NULL,
    venue_id bigint NOT NULL REFERENCES venues (id) ON DELETE RESTRICT,
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL,
    court_count integer NOT NULL CHECK (court_count > 0),
    capacity integer NOT NULL CHECK (capacity > 0),
    price bigint NOT NULL CHECK (price >= 0),
    unique_code_enabled boolean NOT NULL DEFAULT true,
    payment_deadline_at timestamptz,
    refund_deadline_at timestamptz NOT NULL,
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'closed', 'cancelled')),
    point_rule text NOT NULL DEFAULT '',
    level_label text NOT NULL DEFAULT '',
    shuttlecock_included boolean NOT NULL DEFAULT true,
    notes text NOT NULL DEFAULT '',
    court_cost bigint NOT NULL DEFAULT 0,
    shuttlecock_cost bigint NOT NULL DEFAULT 0,
    other_cost bigint NOT NULL DEFAULT 0,
    meta_title text NOT NULL DEFAULT '',
    meta_description text NOT NULL DEFAULT '',
    closed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (ends_at > starts_at)
);
CREATE INDEX sessions_starts_at_idx ON sessions (starts_at);

CREATE TABLE registrations (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    session_id bigint NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    user_id bigint REFERENCES users (id) ON DELETE SET NULL,
    display_name text NOT NULL,
    phone text NOT NULL,
    level text NOT NULL CHECK (level IN ('beginner', 'intermediate', 'advanced')),
    status text NOT NULL CHECK (status IN ('registered', 'waitlisted', 'cancelled', 'transferred', 'attended', 'no_show')),
    payment_status text NOT NULL DEFAULT 'unpaid' CHECK (payment_status IN ('unpaid', 'pending', 'paid', 'rejected', 'refund_pending', 'refunded', 'forfeited')),
    amount_due bigint NOT NULL,
    unique_code integer NOT NULL DEFAULT 0,
    ticket_token_hash bytea UNIQUE,
    registered_at timestamptz NOT NULL DEFAULT now(),
    queued_at timestamptz NOT NULL DEFAULT now(),
    paid_at timestamptz,
    transferred_to_id bigint REFERENCES registrations (id) ON DELETE SET NULL,
    cancelled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (session_id, phone)
);
CREATE INDEX registrations_session_status_idx ON registrations (session_id, status);
CREATE INDEX registrations_user_status_idx ON registrations (user_id, status);
CREATE INDEX registrations_phone_idx ON registrations (phone);

CREATE TABLE payment_proofs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    registration_id bigint NOT NULL REFERENCES registrations (id) ON DELETE CASCADE,
    method text NOT NULL CHECK (method IN ('qris', 'bank_transfer', 'cash')),
    media_id bigint REFERENCES media (id) ON DELETE RESTRICT,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    amount bigint NOT NULL,
    uploaded_at timestamptz NOT NULL DEFAULT now(),
    reviewed_at timestamptz,
    review_note text NOT NULL DEFAULT ''
);
CREATE INDEX payment_proofs_status_idx ON payment_proofs (status, uploaded_at);
CREATE INDEX payment_proofs_registration_idx ON payment_proofs (registration_id);

CREATE TABLE refunds (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    registration_id bigint NOT NULL UNIQUE REFERENCES registrations (id) ON DELETE CASCADE,
    amount bigint NOT NULL,
    bank_name text NOT NULL DEFAULT '',
    account_number text NOT NULL DEFAULT '',
    account_name text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed')),
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
CREATE INDEX refunds_status_idx ON refunds (status, created_at);

CREATE TABLE rotation_rounds (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    session_id bigint NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    round_number integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (session_id, round_number)
);

CREATE TABLE rotation_matches (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    round_id bigint NOT NULL REFERENCES rotation_rounds (id) ON DELETE CASCADE,
    court_number integer NOT NULL,
    team_a_player1_id bigint NOT NULL REFERENCES registrations (id) ON DELETE CASCADE,
    team_a_player2_id bigint NOT NULL REFERENCES registrations (id) ON DELETE CASCADE,
    team_b_player1_id bigint NOT NULL REFERENCES registrations (id) ON DELETE CASCADE,
    team_b_player2_id bigint NOT NULL REFERENCES registrations (id) ON DELETE CASCADE
);
CREATE INDEX rotation_matches_round_idx ON rotation_matches (round_id);

CREATE TABLE pages (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug text NOT NULL UNIQUE,
    title text NOT NULL,
    body_markdown text NOT NULL DEFAULT '',
    cover_media_id bigint REFERENCES media (id) ON DELETE RESTRICT,
    is_published boolean NOT NULL DEFAULT false,
    show_in_footer boolean NOT NULL DEFAULT false,
    sort_order integer NOT NULL DEFAULT 0,
    meta_title text NOT NULL DEFAULT '',
    meta_description text NOT NULL DEFAULT '',
    meta_keywords text NOT NULL DEFAULT '',
    og_media_id bigint REFERENCES media (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE faq_items (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    question text NOT NULL,
    answer_markdown text NOT NULL,
    sort_order integer NOT NULL DEFAULT 0,
    is_published boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE hero_slides (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    image_media_id bigint NOT NULL REFERENCES media (id) ON DELETE RESTRICT,
    mobile_image_media_id bigint REFERENCES media (id) ON DELETE RESTRICT,
    title text NOT NULL DEFAULT '',
    subtitle text NOT NULL DEFAULT '',
    cta_label text NOT NULL DEFAULT '',
    cta_url text NOT NULL DEFAULT '',
    sort_order integer NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT true,
    starts_at timestamptz,
    ends_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE announcements (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    message text NOT NULL CHECK (char_length(message) <= 160),
    link_url text NOT NULL DEFAULT '',
    style text NOT NULL DEFAULT 'info' CHECK (style IN ('info', 'important')),
    starts_at timestamptz,
    ends_at timestamptz,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE slug_redirects (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    entity_type text NOT NULL CHECK (entity_type IN ('page', 'session')),
    old_slug text NOT NULL,
    new_slug text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (entity_type, old_slug)
);

CREATE TABLE activity_logs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_type text NOT NULL CHECK (actor_type IN ('admin', 'member', 'guest', 'system')),
    actor_id bigint,
    entity_type text NOT NULL,
    entity_id bigint NOT NULL,
    action text NOT NULL,
    data jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX activity_logs_entity_idx ON activity_logs (entity_type, entity_id);

-- +goose Down
DROP TABLE activity_logs;
DROP TABLE slug_redirects;
DROP TABLE announcements;
DROP TABLE hero_slides;
DROP TABLE faq_items;
DROP TABLE pages;
DROP TABLE rotation_matches;
DROP TABLE rotation_rounds;
DROP TABLE refunds;
DROP TABLE payment_proofs;
DROP TABLE registrations;
DROP TABLE sessions;
DROP TABLE session_templates;
DROP TABLE venues;
DROP TABLE settings;
DROP TABLE auth_sessions;
DROP TABLE users;
DROP TABLE media;
