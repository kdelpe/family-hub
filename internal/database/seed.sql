-- The made-up family from the mockups (design-notes §7), for local dev: `make seed`. Never real relatives.
-- Sign in as maya.pluviose@example.com (admin) with the dev password `sample-family-dev`; Nadège is the other
-- admin, and Fritz, Naïka, Jean-Robert, Mackenson and Stanley are members, all with the same password.
BEGIN;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM persons) OR EXISTS (SELECT 1 FROM users) THEN
    RAISE EXCEPTION 'seed: the database already has people or users; it only loads into an empty one';
  END IF;
END $$;

-- Ids end in a short key, so the rows below can name people by it.
CREATE TEMP TABLE seed_person (k text PRIMARY KEY, id uuid NOT NULL) ON COMMIT DROP;
INSERT INTO seed_person
SELECT k, ('019a0000-0000-7000-8000-0000000000' || k)::uuid
FROM unnest(ARRAY['01','02','03','04','05','06','07','08','09','0a','0b','0c','0d','0e','0f','10','11','12','13','14','15','16','17','18','19','1a']) k;

INSERT INTO persons (id, first_name, last_name, maiden_name, side, gender, birth_date, birth_date_precision,
                     death_date, death_date_precision, is_deceased, bio)
SELECT s.id, p.first, p.last, p.maiden, p.side, p.gender, p.born::date, p.born_precision, p.died::date, p.died_precision,
       p.died IS NOT NULL, p.bio
FROM (VALUES
  ('01', 'Jean-Baptiste', 'Pluviose', NULL, 'first', 'male', '1900-01-01', 'circa', '1971-01-01', 'year',
   'Ran a small bakery near the market in Jacmel. Still remembered for his pen mayi.'),
  ('02', 'Anne-Marie', 'Pluviose', NULL, 'first', 'female', '1904-01-01', 'circa', '1980-01-01', 'year',
   'Remembered for her garden and her strict rules about Sunday lunch.'),
  ('03', 'Joseph', 'Pluviose', NULL, 'first', 'male', '1929-01-01', 'year', '2004-01-01', 'year',
   'Drove a yellow cab in New York for 30 years. His old taxi medallion hangs in Fritz’s living room.'),
  ('04', 'Marie-Claire', 'Pluviose', NULL, 'first', 'female', '1933-01-01', 'year', '2015-01-01', 'year',
   'Sang in the church choir until she was 80.'),
  ('05', 'Lucien', 'Monestime', NULL, 'second', 'male', '1935-01-01', 'circa', '1998-01-01', 'year',
   'Carpenter. Built the Monestime house on Rue Sainte-Anne himself.'),
  ('06', 'Rose', 'Monestime', NULL, 'second', 'female', '1937-02-02', 'day', NULL, NULL,
   'Keeper of the pen patat recipe. Still talks about the house Lucien built in Jacmel.'),
  ('07', 'Jean-Robert', 'Pluviose', NULL, 'first', 'male', '1953-08-12', 'day', NULL, NULL,
   'Retired high-school band director. Plays trumpet at every wedding.'),
  ('08', 'Nadège', 'Pluviose', 'Monestime', 'second', 'female', '1956-11-19', 'day', NULL, NULL,
   'Grew up in Brooklyn, taught third grade for thirty years, and still makes the best soup joumou every New Year’s Day.'),
  ('09', 'Guerline', 'Saint-Fleur', NULL, 'married_in', 'female', '1958-10-03', 'day', NULL, NULL,
   'Sabine’s mother. Lives in Montreal.'),
  ('0a', 'Marie-Thérèse', 'Pluviose', NULL, 'first', 'female', '1956-03-15', 'day', NULL, NULL,
   'Jean-Robert’s younger sister. Runs the family reunion every other summer.'),
  ('0b', 'Peterson', 'Pluviose', NULL, 'first', 'male', '1984-08-30', 'day', NULL, NULL, 'Marie-Thérèse’s son.'),
  ('0c', 'Frantz', 'Monestime', NULL, 'second', 'male', '1960-12-01', 'day', NULL, NULL,
   'Nadège’s brother. Took over Lucien’s carpentry shop.'),
  ('0d', 'Mirlande', 'Monestime', NULL, 'married_in', 'female', '1962-06-18', 'day', NULL, NULL, 'Married Frantz in 1990.'),
  ('0e', 'Darline', 'Monestime', NULL, 'second', 'female', '1991-02-14', 'day', NULL, NULL, 'Frantz and Mirlande’s daughter.'),
  ('0f', 'Maya', 'Pluviose', NULL, 'both', 'female', '1978-06-06', 'day', NULL, NULL,
   'Eldest of the Pluviose kids. Nurse in Boston. Keeper of the family recipes.'),
  ('10', 'Stanley', 'Jean-Louis', NULL, 'married_in', 'male', '1976-09-09', 'day', NULL, NULL,
   'Grew up in Cap-Haïtien. Makes the best diri djon djon.'),
  ('11', 'Naïka', 'Jean-Louis', NULL, 'both', 'female', '2001-04-08', 'day', NULL, NULL, 'Architect in Atlanta.'),
  ('12', 'Mackenson', 'Jean-Louis', NULL, 'both', 'male', '2004-11-24', 'day', NULL, NULL,
   'Plays bass in a band with three of his cousins.'),
  ('13', 'Ralph', 'Désir', NULL, 'married_in', 'male', '2000-01-11', 'day', NULL, NULL, 'Met Naïka in architecture school.'),
  ('14', 'Naomie', 'Désir', NULL, 'both', 'female', '2025-07-02', 'day', NULL, NULL,
   'Naïka and Ralph’s daughter — the first of the fifth generation.'),
  ('15', 'Fritz', 'Pluviose', NULL, 'both', 'male', '1980-01-30', 'day', NULL, NULL,
   'Hosts Thanksgiving. Keeps Joseph’s old taxi medallion.'),
  ('16', 'Fabienne', 'Pluviose', NULL, 'married_in', 'female', '1981-04-27', 'day', NULL, NULL, 'Married Fritz in 2004.'),
  ('17', 'Kervens', 'Pluviose', NULL, 'both', 'male', '2006-03-03', 'day', NULL, NULL, 'Fritz and Fabienne’s eldest.'),
  ('18', 'Esther', 'Pluviose', NULL, 'both', 'female', '2009-09-17', 'day', NULL, NULL, 'Fritz and Fabienne’s middle child.'),
  ('19', 'Jonas', 'Pluviose', NULL, 'both', 'male', '2012-12-22', 'day', NULL, NULL, 'Fritz and Fabienne’s youngest.'),
  ('1a', 'Sabine', 'Pluviose', NULL, 'first', 'female', '1986-05-21', 'day', NULL, NULL,
   'Jean-Robert and Guerline’s daughter. Teaches high-school chemistry.')
) AS p(k, first, last, maiden, side, gender, born, born_precision, died, died_precision, bio)
JOIN seed_person s USING (k);

-- Parents, then couples. A couple's ids are sorted, as the schema requires.
INSERT INTO relationships (type, subtype, person_a, person_b)
SELECT 'parent_child', 'biological', parent.id, child.id
FROM (VALUES
  ('01','03'), ('02','03'), ('03','07'), ('04','07'), ('03','0a'), ('04','0a'),
  ('05','08'), ('06','08'), ('05','0c'), ('06','0c'),
  ('07','0f'), ('08','0f'), ('07','15'), ('08','15'), ('07','1a'), ('09','1a'),
  ('0a','0b'), ('0c','0e'), ('0d','0e'),
  ('0f','11'), ('10','11'), ('0f','12'), ('10','12'), ('11','14'), ('13','14'),
  ('15','17'), ('16','17'), ('15','18'), ('16','18'), ('15','19'), ('16','19')
) AS r(p, c)
JOIN seed_person parent ON parent.k = r.p JOIN seed_person child ON child.k = r.c;

INSERT INTO relationships (type, subtype, ended, person_a, person_b)
SELECT 'couple', r.subtype, r.ended, least(a.id, b.id), greatest(a.id, b.id)
FROM (VALUES
  ('01','02','married',false), ('03','04','married',false), ('05','06','married',false), ('07','08','married',false),
  ('07','09','married',true), ('0c','0d','married',false), ('0f','10','married',false), ('15','16','married',false),
  ('11','13','partner',false)
) AS r(x, y, subtype, ended)
JOIN seed_person a ON a.k = r.x JOIN seed_person b ON b.k = r.y;

-- The Members & invites list: everyone signed up shares the dev password; Sabine and Rose were invited and
-- haven't set one yet.
INSERT INTO users (email, role, person_id, password_hash, password_changed_at)
SELECT u.email, u.role, s.id, CASE WHEN u.active THEN
  '$argon2id$v=19$m=19456,t=2,p=1$8olbmBbyQUOCTh2R/l6EDA$RwSZHioctZ59fvax9nrTT/cvdg9yCUg2WaacpaEfMmA' END,
  CASE WHEN u.active THEN now() END
FROM (VALUES
  ('0f', 'maya.pluviose@example.com', 'admin', true),
  ('08', 'nadege.p@example.com', 'admin', true),
  ('15', 'fritz.p@example.com', 'member', true),
  ('11', 'naika@example.com', 'member', true),
  ('07', 'jean-robert.p@example.com', 'member', true),
  ('12', 'mackenson.jl@example.com', 'member', true),
  ('10', 'stanley.jl@example.com', 'member', true),
  ('1a', 'sabine.p@example.com', 'member', false),
  ('06', 'rose.monestime@example.com', 'member', false)
) AS u(k, email, role, active)
JOIN seed_person s USING (k);

-- Maya's from My profile, the rest from the Directory.
INSERT INTO contact_methods (person_id, kind, platform, label, value, visibility)
SELECT s.id, c.kind, c.platform, c.label, c.value, c.visibility
FROM (VALUES
  ('0f', 'phone', NULL, 'Mobile', '(555) 010-7781', 'on_request'),
  ('0f', 'email', NULL, NULL, 'maya.pluviose@example.com', 'family'),
  ('0f', 'address', NULL, 'Home', '88 Linden St, Boston, MA', 'hidden'),
  ('0f', 'social', 'whatsapp', NULL, '(555) 010-7781', 'on_request'),
  ('0f', 'social', 'instagram', NULL, '@maya.pluviose', 'hidden'),
  ('0f', 'social', 'facebook', NULL, 'facebook.com/maya.pluviose', 'hidden'),
  ('15', 'email', NULL, NULL, 'fritz.p@example.com', 'family'),
  ('15', 'social', 'facebook', NULL, 'facebook.com/fritz.pluviose', 'family'),
  ('15', 'social', 'youtube', NULL, 'youtube.com/@fritzpluviose', 'family'),
  ('15', 'phone', NULL, 'Mobile', '(555) 010-2231', 'on_request'),
  ('15', 'address', NULL, 'Home', '14 Maple Ave, Brooklyn, NY', 'on_request'),
  ('07', 'phone', NULL, 'Mobile', '(555) 010-4120', 'on_request'),
  ('07', 'address', NULL, 'Home', '301 Flatbush Ave, Brooklyn, NY', 'on_request'),
  ('12', 'social', 'tiktok', NULL, '@mackensonjl', 'family'),
  ('12', 'social', 'instagram', NULL, '@mackenson.jl', 'family'),
  ('12', 'social', 'youtube', NULL, 'youtube.com/@mackensonjl', 'family'),
  ('12', 'phone', NULL, 'Mobile', '(555) 010-6604', 'on_request'),
  ('08', 'social', 'facebook', NULL, 'facebook.com/nadege.pluviose', 'family'),
  ('08', 'phone', NULL, 'Mobile', '(555) 010-4121', 'on_request'),
  ('08', 'social', 'whatsapp', NULL, '(555) 010-4121', 'on_request'),
  ('08', 'address', NULL, 'Home', '301 Flatbush Ave, Brooklyn, NY', 'on_request'),
  ('11', 'email', NULL, NULL, 'naika@example.com', 'family'),
  ('11', 'social', 'instagram', NULL, '@naika.jl', 'family'),
  ('11', 'social', 'x', NULL, '@naikajl', 'family'),
  ('11', 'phone', NULL, 'Mobile', '(555) 010-8852', 'on_request'),
  ('06', 'phone', NULL, 'Home', '(555) 010-3307', 'on_request'),
  ('10', 'email', NULL, NULL, 'stanley.jl@example.com', 'family'),
  ('10', 'social', 'instagram', NULL, '@stanley.jl', 'family')
) AS c(k, kind, platform, label, value, visibility)
JOIN seed_person s USING (k);

-- The Requests queue.
INSERT INTO access_requests (full_name, email, side, relationship, related_names, note, source, suggested_by, created_at)
VALUES
  ('Darline Monestime', 'darline.monestime@example.com', 'second', 'daughter', '{"Frantz Monestime", "Mirlande Monestime"}',
   'Cousin Maya told me about the site at Thanksgiving.', 'self', NULL, now() - interval '2 days'),
  ('Wesner Jean-Louis', 'wesner.jl@example.com', 'married_in', 'brother', '{"Stanley Jean-Louis"}',
   'He has a box of old photos from Cap-Haïtien he wants to share.', 'member_suggestion',
   (SELECT id FROM users WHERE email = 'stanley.jl@example.com'), now() - interval '1 day');

COMMIT;
