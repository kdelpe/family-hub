-- The made-up family from the mockups (design-notes §7), for local dev: `make seed`. Never real relatives.
-- Fixed ids and ON CONFLICT DO NOTHING, so running it again adds nothing. It creates no admin: run
-- `admin create`, then link your login to Maya Pluviose from Members & invites (PATCH /api/admin/users/{id}).
BEGIN;

INSERT INTO persons (id, first_name, last_name, maiden_name, side, gender, birth_date, birth_date_precision,
                     death_date, death_date_precision, is_deceased, birth_place, bio) VALUES
('019a0000-0000-7000-8000-000000000001', 'Jean-Baptiste', 'Pluviose', NULL, 'first', 'male', '1898-01-01', 'circa', '1961-01-01', 'year', true, 'Jacmel', NULL),
('019a0000-0000-7000-8000-000000000002', 'Anne-Marie', 'Pluviose', NULL, 'first', 'female', '1902-01-01', 'circa', '1970-01-01', 'year', true, 'Jacmel', NULL),
('019a0000-0000-7000-8000-000000000003', 'Joseph', 'Pluviose', NULL, 'first', 'male', '1924-05-02', 'day', '1998-11-20', 'day', true, 'Jacmel',
 'Ran the bakery on Rue du Commerce for forty years.'),
('019a0000-0000-7000-8000-000000000004', 'Marie-Claire', 'Pluviose', NULL, 'first', 'female', '1929-08-01', 'month', NULL, NULL, false, 'Léogâne', NULL),
('019a0000-0000-7000-8000-000000000005', 'Lucien', 'Monestime', NULL, 'second', 'male', '1926-01-01', 'year', '2004-01-01', 'year', true, 'Cap-Haïtien', NULL),
('019a0000-0000-7000-8000-000000000006', 'Rose', 'Monestime', NULL, 'second', 'female', '1931-04-17', 'day', NULL, NULL, false, 'Cap-Haïtien', NULL),
('019a0000-0000-7000-8000-000000000007', 'Jean-Robert', 'Pluviose', NULL, 'first', 'male', '1950-09-23', 'day', NULL, NULL, false, 'Jacmel', NULL),
('019a0000-0000-7000-8000-000000000008', 'Nadège', 'Pluviose', 'Monestime', 'second', 'female', '1953-02-11', 'day', NULL, NULL, false, 'Cap-Haïtien', NULL),
('019a0000-0000-7000-8000-000000000009', 'Guerline', 'Saint-Fleur', NULL, 'married_in', 'female', '1955-01-01', 'year', NULL, NULL, false, NULL, NULL),
('019a0000-0000-7000-8000-00000000000a', 'Marie-Thérèse', 'Pluviose', NULL, 'first', 'female', '1952-06-01', 'month', NULL, NULL, false, 'Jacmel', NULL),
('019a0000-0000-7000-8000-00000000000b', 'Peterson', 'Pluviose', NULL, 'first', 'male', '1980-10-05', 'day', NULL, NULL, false, NULL, NULL),
('019a0000-0000-7000-8000-00000000000c', 'Frantz', 'Monestime', NULL, 'second', 'male', '1956-12-30', 'day', NULL, NULL, false, 'Cap-Haïtien', NULL),
('019a0000-0000-7000-8000-00000000000d', 'Mirlande', 'Monestime', NULL, 'married_in', 'female', '1958-01-01', 'year', NULL, NULL, false, NULL, NULL),
('019a0000-0000-7000-8000-00000000000e', 'Darline', 'Monestime', NULL, 'second', 'female', '1984-07-19', 'day', NULL, NULL, false, 'Miami', NULL),
('019a0000-0000-7000-8000-00000000000f', 'Maya', 'Pluviose', NULL, 'both', 'female', '1975-03-12', 'day', NULL, NULL, false, 'Port-au-Prince',
 'Keeper of the family photos and the one who started this hub.'),
('019a0000-0000-7000-8000-000000000010', 'Stanley', 'Jean-Louis', NULL, 'married_in', 'male', '1973-11-02', 'day', NULL, NULL, false, NULL, NULL),
('019a0000-0000-7000-8000-000000000011', 'Naïka', 'Jean-Louis', NULL, 'both', 'female', '1999-05-24', 'day', NULL, NULL, false, 'Boston', NULL),
('019a0000-0000-7000-8000-000000000012', 'Mackenson', 'Jean-Louis', NULL, 'both', 'male', '2003-08-09', 'day', NULL, NULL, false, 'Boston', NULL),
('019a0000-0000-7000-8000-000000000013', 'Ralph', 'Désir', NULL, 'married_in', 'male', '1997-01-01', 'year', NULL, NULL, false, NULL, NULL),
('019a0000-0000-7000-8000-000000000014', 'Naomie', 'Désir', NULL, 'both', 'female', '2024-02-14', 'day', NULL, NULL, false, 'Boston', NULL),
('019a0000-0000-7000-8000-000000000015', 'Fritz', 'Pluviose', NULL, 'both', 'male', '1978-04-30', 'day', NULL, NULL, false, 'Port-au-Prince', NULL),
('019a0000-0000-7000-8000-000000000016', 'Fabienne', 'Pluviose', NULL, 'married_in', 'female', '1980-01-01', 'year', NULL, NULL, false, NULL, NULL),
('019a0000-0000-7000-8000-000000000017', 'Kervens', 'Pluviose', NULL, 'both', 'male', '2005-09-15', 'day', NULL, NULL, false, NULL, NULL),
('019a0000-0000-7000-8000-000000000018', 'Esther', 'Pluviose', NULL, 'both', 'female', '2008-01-01', 'year', NULL, NULL, false, NULL, NULL),
('019a0000-0000-7000-8000-000000000019', 'Jonas', 'Pluviose', NULL, 'both', 'male', '2011-06-01', 'month', NULL, NULL, false, NULL, NULL),
('019a0000-0000-7000-8000-00000000001a', 'Sabine', 'Pluviose', NULL, 'first', 'female', '1985-10-21', 'day', NULL, NULL, false, NULL, NULL)
ON CONFLICT (id) DO NOTHING;

-- parent_child: person_a is the parent. couple: person_a < person_b.
INSERT INTO relationships (type, subtype, ended, person_a, person_b)
SELECT t, s, e, ('019a0000-0000-7000-8000-0000000000' || a)::uuid, ('019a0000-0000-7000-8000-0000000000' || b)::uuid
FROM (VALUES
  ('couple', 'married', false, '01', '02'), ('parent_child', 'biological', false, '01', '03'), ('parent_child', 'biological', false, '02', '03'),
  ('couple', 'married', false, '03', '04'),
  ('parent_child', 'biological', false, '03', '07'), ('parent_child', 'biological', false, '04', '07'),
  ('parent_child', 'biological', false, '03', '0a'), ('parent_child', 'biological', false, '04', '0a'),
  ('couple', 'married', false, '05', '06'),
  ('parent_child', 'biological', false, '05', '08'), ('parent_child', 'biological', false, '06', '08'),
  ('parent_child', 'biological', false, '05', '0c'), ('parent_child', 'biological', false, '06', '0c'),
  ('couple', 'married', false, '07', '08'),
  ('parent_child', 'biological', false, '07', '0f'), ('parent_child', 'biological', false, '08', '0f'),
  ('parent_child', 'biological', false, '07', '15'), ('parent_child', 'biological', false, '08', '15'),
  ('couple', 'partner', true, '07', '09'),
  ('parent_child', 'biological', false, '07', '1a'), ('parent_child', 'biological', false, '09', '1a'),
  ('parent_child', 'biological', false, '0a', '0b'),
  ('couple', 'married', false, '0c', '0d'),
  ('parent_child', 'biological', false, '0c', '0e'), ('parent_child', 'biological', false, '0d', '0e'),
  ('couple', 'married', false, '0f', '10'),
  ('parent_child', 'biological', false, '0f', '11'), ('parent_child', 'biological', false, '10', '11'),
  ('parent_child', 'biological', false, '0f', '12'), ('parent_child', 'biological', false, '10', '12'),
  ('couple', 'married', false, '11', '13'),
  ('parent_child', 'biological', false, '11', '14'), ('parent_child', 'biological', false, '13', '14'),
  ('couple', 'married', false, '15', '16'),
  ('parent_child', 'biological', false, '15', '17'), ('parent_child', 'biological', false, '16', '17'),
  ('parent_child', 'biological', false, '15', '18'), ('parent_child', 'biological', false, '16', '18'),
  ('parent_child', 'biological', false, '15', '19'), ('parent_child', 'biological', false, '16', '19')
) AS r(t, s, e, a, b)
ON CONFLICT (type, person_a, person_b) DO NOTHING;

-- Two members who were invited but haven't set a password, so on-request contacts have someone to answer
-- and the Requests queue has a suggester.
INSERT INTO users (id, email, role, person_id) VALUES
('019a0000-0000-7000-8001-000000000010', 'stanley@example.com', 'member', '019a0000-0000-7000-8000-000000000010'),
('019a0000-0000-7000-8001-00000000000e', 'darline@example.com', 'member', '019a0000-0000-7000-8000-00000000000e')
ON CONFLICT DO NOTHING;

INSERT INTO contact_methods (id, person_id, kind, platform, label, value, visibility) VALUES
('019a0000-0000-7000-8002-000000000001', '019a0000-0000-7000-8000-00000000000f', 'phone', NULL, 'Mobile', '(555) 010-7781', 'on_request'),
('019a0000-0000-7000-8002-000000000002', '019a0000-0000-7000-8000-00000000000f', 'email', NULL, NULL, 'maya.pluviose@example.com', 'family'),
('019a0000-0000-7000-8002-000000000003', '019a0000-0000-7000-8000-00000000000f', 'address', NULL, 'Home', '88 Linden St, Boston, MA', 'on_request'),
('019a0000-0000-7000-8002-000000000004', '019a0000-0000-7000-8000-00000000000f', 'social', 'whatsapp', NULL, '(555) 010-7781', 'on_request'),
('019a0000-0000-7000-8002-000000000005', '019a0000-0000-7000-8000-00000000000f', 'social', 'instagram', NULL, '@maya.pluviose', 'hidden'),
('019a0000-0000-7000-8002-000000000006', '019a0000-0000-7000-8000-000000000010', 'email', NULL, NULL, 'stanley.jeanlouis@example.com', 'family'),
('019a0000-0000-7000-8002-000000000007', '019a0000-0000-7000-8000-000000000010', 'social', 'facebook', NULL, 'facebook.com/stanley.jeanlouis', 'family'),
('019a0000-0000-7000-8002-000000000008', '019a0000-0000-7000-8000-000000000010', 'phone', NULL, 'Mobile', '(555) 010-4410', 'on_request'),
('019a0000-0000-7000-8002-000000000009', '019a0000-0000-7000-8000-00000000000e', 'email', NULL, NULL, 'darline.monestime@example.com', 'family'),
('019a0000-0000-7000-8002-00000000000a', '019a0000-0000-7000-8000-00000000000e', 'address', NULL, 'Home', '12 Palm Ct, Miami, FL', 'on_request'),
('019a0000-0000-7000-8002-00000000000b', '019a0000-0000-7000-8000-000000000011', 'social', 'instagram', NULL, '@naika.jl', 'family'),
('019a0000-0000-7000-8002-00000000000c', '019a0000-0000-7000-8000-000000000015', 'email', NULL, NULL, 'fritz.pluviose@example.com', 'family')
ON CONFLICT (id) DO NOTHING;

-- One request from the sign-in page and one suggested by Stanley, for the Requests queue.
INSERT INTO access_requests (id, full_name, email, side, relationship, related_names, note, source, suggested_by) VALUES
('019a0000-0000-7000-8003-000000000001', 'Kervens Pluviose', 'kervens@example.com', 'first', 'son',
 '{"Fritz Pluviose", "Fabienne Pluviose"}', NULL, 'member_suggestion', '019a0000-0000-7000-8001-000000000010'),
('019a0000-0000-7000-8003-000000000002', 'Kettly Monestime', 'kettly@example.com', 'second', 'cousin',
 '{"Frantz Monestime", "Lucien Monestim"}', 'Darline told me about the hub.', 'self', NULL)
ON CONFLICT DO NOTHING;

COMMIT;
