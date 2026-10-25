-- migrations/000_baseline_schema.sql
-- Baseline schema captured from production on 2026-10-24.
-- Contains the 5 tables that predate the migration system:
--   business_profiles, clients, invoice_items, invoices, users
-- All later migrations (001+) build on top of this.
--
-- Safety: CREATE TABLE IF NOT EXISTS and CREATE SEQUENCE IF NOT EXISTS
-- make this a no-op if the tables already exist (as on production).
-- Constraints are wrapped in DO blocks for the same reason.
--
-- To mark as already-applied on an existing database:
--   INSERT INTO schema_migrations (filename) VALUES ('000_baseline_schema.sql')
--   ON CONFLICT (filename) DO NOTHING;

-- Name: business_profiles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS business_profiles (
    id integer NOT NULL,
    name text NOT NULL,
    email text,
    address text,
    tax_id text,
    default_tax_rate numeric(6,3) DEFAULT 0,
    currency text DEFAULT 'USD'::text,
    logo_url text,
    created_at timestamp with time zone DEFAULT now(),
    user_id integer,
    city text,
    state text,
    zip text,
    country text,
    phone text DEFAULT ''::text NOT NULL,
    slug text DEFAULT ''::text NOT NULL,
    zelle_id text DEFAULT ''::text NOT NULL,
    venmo_handle text DEFAULT ''::text NOT NULL,
    cashapp_handle text DEFAULT ''::text NOT NULL,
    service_areas text DEFAULT ''::text NOT NULL,
    invoice_template character varying(20) DEFAULT 'standard'::character varying NOT NULL
);


--
-- Name: business_profiles_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS business_profiles_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: business_profiles_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE business_profiles_id_seq OWNED BY business_profiles.id;


--
-- Name: clients; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS clients (
    id integer NOT NULL,
    business_profile_id integer NOT NULL,
    name text NOT NULL,
    email text,
    address text,
    default_tax_rate numeric(6,3),
    payment_terms text,
    created_at timestamp with time zone DEFAULT now(),
    city text,
    state text,
    zip text,
    country text,
    phone text,
    notes text
);


--
-- Name: clients_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS clients_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: clients_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE clients_id_seq OWNED BY clients.id;


--
-- Name: invoice_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS invoice_items (
    id integer NOT NULL,
    invoice_id integer NOT NULL,
    description text NOT NULL,
    quantity numeric(12,3) DEFAULT 1 NOT NULL,
    unit_price numeric(12,2) DEFAULT 0 NOT NULL,
    line_total numeric(12,2) DEFAULT 0,
    unit_price_cents bigint DEFAULT 0 NOT NULL,
    line_total_cents bigint DEFAULT 0 NOT NULL,
    details text DEFAULT ''::text NOT NULL
);


--
-- Name: invoice_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS invoice_items_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: invoice_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE invoice_items_id_seq OWNED BY invoice_items.id;


--
-- Name: invoices; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS invoices (
    id integer NOT NULL,
    business_profile_id integer,
    client_id integer,
    invoice_number text NOT NULL,
    issue_date date DEFAULT CURRENT_DATE NOT NULL,
    due_date date,
    status text DEFAULT 'draft'::text NOT NULL,
    tax_rate numeric(6,3) DEFAULT 0 NOT NULL,
    discount_amount numeric(12,2) DEFAULT 0,
    notes text,
    payment_details text,
    pdf_url text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now(),
    currency text DEFAULT 'USD'::text,
    client_name text DEFAULT ''::text NOT NULL,
    subtotal numeric(12,2) DEFAULT 0 NOT NULL,
    tax_amount numeric(12,2) DEFAULT 0 NOT NULL,
    total numeric(12,2) DEFAULT 0 NOT NULL,
    company_name text,
    company_email text,
    company_address text,
    company_city text,
    company_zip text,
    company_country text,
    company_state text,
    client_email text,
    client_address text,
    client_city text,
    client_state text,
    client_country text,
    client_zip text,
    user_id integer,
    tax_rate_bps bigint DEFAULT 0 NOT NULL,
    discount_amount_cents bigint DEFAULT 0 NOT NULL,
    subtotal_cents bigint DEFAULT 0 NOT NULL,
    tax_amount_cents bigint DEFAULT 0 NOT NULL,
    total_cents bigint DEFAULT 0 NOT NULL,
    anonymous_token character varying(64) DEFAULT NULL::character varying,
    logo_url text,
    show_logo boolean DEFAULT true NOT NULL,
    show_title boolean DEFAULT true NOT NULL,
    auto_reminders boolean DEFAULT false NOT NULL,
    template_id character varying(20) DEFAULT 'classic'::character varying NOT NULL,
    brand_color character varying(7) DEFAULT '#0d1422'::character varying NOT NULL,
    public_token character varying(64),
    document_type character varying(20) DEFAULT 'invoice'::character varying NOT NULL,
    logo_position character varying(10) DEFAULT 'left'::character varying NOT NULL,
    payment_method text DEFAULT ''::text NOT NULL,
    payment_reported_at timestamp with time zone,
    payment_reported_method text DEFAULT ''::text,
    payment_reported_note text DEFAULT ''::text
);


--
-- Name: invoices_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS invoices_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: invoices_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE invoices_id_seq OWNED BY invoices.id;


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS users (
    id integer NOT NULL,
    email text NOT NULL,
    password_hash text,
    plan text DEFAULT 'free'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    provider text DEFAULT 'email'::text NOT NULL,
    google_id text,
    name text,
    avatar_url text,
    stripe_customer_id text,
    stripe_connect_id character varying(255),
    next_invoice_seq integer DEFAULT 1 NOT NULL,
    next_estimate_seq integer DEFAULT 1 NOT NULL,
    language character varying(5) DEFAULT 'en'::character varying NOT NULL,
    magic_token text,
    magic_token_expires_at timestamp with time zone,
    password_algo text DEFAULT 'bcrypt'::text NOT NULL,
    failed_login_attempts integer DEFAULT 0 NOT NULL,
    locked_until timestamp with time zone,
    setup_step integer DEFAULT 0 NOT NULL
);


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE IF NOT EXISTS users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE users_id_seq OWNED BY users.id;


--
-- Name: business_profiles id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY business_profiles ALTER COLUMN id SET DEFAULT nextval('business_profiles_id_seq'::regclass);


--
-- Name: clients id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY clients ALTER COLUMN id SET DEFAULT nextval('clients_id_seq'::regclass);


--
-- Name: invoice_items id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY invoice_items ALTER COLUMN id SET DEFAULT nextval('invoice_items_id_seq'::regclass);


--
-- Name: invoices id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY invoices ALTER COLUMN id SET DEFAULT nextval('invoices_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY users ALTER COLUMN id SET DEFAULT nextval('users_id_seq'::regclass);


--
-- Name: business_profiles business_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY business_profiles
    ADD CONSTRAINT business_profiles_pkey PRIMARY KEY (id);


--
-- Name: clients clients_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY clients
    ADD CONSTRAINT clients_pkey PRIMARY KEY (id);


--
-- Name: invoice_items invoice_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY invoice_items
    ADD CONSTRAINT invoice_items_pkey PRIMARY KEY (id);


--
-- Name: invoices invoices_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY invoices
    ADD CONSTRAINT invoices_pkey PRIMARY KEY (id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: business_profiles business_profiles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY business_profiles
    ADD CONSTRAINT business_profiles_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;


--
-- Name: clients clients_business_profile_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY clients
    ADD CONSTRAINT clients_business_profile_id_fkey FOREIGN KEY (business_profile_id) REFERENCES business_profiles(id) ON DELETE CASCADE;


--
-- Name: invoice_items invoice_items_invoice_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY invoice_items
    ADD CONSTRAINT invoice_items_invoice_id_fkey FOREIGN KEY (invoice_id) REFERENCES invoices(id) ON DELETE CASCADE;


--
-- Name: invoices invoices_business_profile_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY invoices
    ADD CONSTRAINT invoices_business_profile_id_fkey FOREIGN KEY (business_profile_id) REFERENCES business_profiles(id) ON DELETE CASCADE;


--
-- Name: invoices invoices_client_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY invoices
    ADD CONSTRAINT invoices_client_id_fkey FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE SET NULL;


--
-- Name: invoices invoices_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY invoices
    ADD CONSTRAINT invoices_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;


--
