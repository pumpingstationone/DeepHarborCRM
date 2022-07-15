DROP SEQUENCE person_id_seq;
CREATE SEQUENCE person_id_seq INCREMENT BY 1 MINVALUE 1 MAXVALUE 2147483647 START WITH 1  NO CYCLE;
DROP TABLE enctest;
CREATE TABLE enctest (id INTEGER, name TEXT);
DROP TABLE orders;
CREATE TABLE orders (id INTEGER NOT NULL, parent_id INTEGER, name TEXT NOT NULL, topic TEXT);
DROP TABLE person;
CREATE TABLE person (id SERIAL NOT NULL, name JSONB NOT NULL, connections JSONB NOT NULL, address JSONB NOT NULL, contact JSONB NOT NULL, status JSONB NOT NULL, forms JSONB, access JSONB, authorizations JSONB, storage_area JSONB, date_added TIMESTAMP(6) WITHOUT TIME ZONE NOT NULL, date_modified TIMESTAMP(6) WITHOUT TIME ZONE NOT NULL, PRIMARY KEY (id));
DROP TABLE person_audit;
CREATE TABLE person_audit (id INTEGER NOT NULL, name JSONB NOT NULL, connections JSONB NOT NULL, address JSONB NOT NULL, contact JSONB NOT NULL, status JSONB NOT NULL, forms JSONB, access JSONB, authorizations JSONB, storage_area JSONB, version INTEGER NOT NULL, date_added TIMESTAMP(6) WITHOUT TIME ZONE NOT NULL);
DROP TABLE wapersondata;
CREATE TABLE wapersondata (person_id INTEGER NOT NULL, wadata JSONB NOT NULL);
ALTER TABLE "wapersondata" ADD CONSTRAINT wapersondata_fk1 FOREIGN KEY ("person_id") REFERENCES "person" ("id");
DROP FUNCTION add_person_audit_record ();
--/
CREATE FUNCTION add_person_audit_record ()  RETURNS trigger
  VOLATILE
AS $body$
BEGIN
INSERT INTO 
    person_audit 
    ( 
        id, 
        NAME, 
        connections, 
        address, 
        contact, 
        status, 
        forms, 
        ACCESS, 
        authorizations, 
        storage_area, 
        VERSION, 
        date_added 
    )
    VALUES 
    ( 
        new.id, 
        new.name, 
        new.connections, 
        new.address, 
        new.contact, 
        new.status, 
        new.forms, 
        new.access, 
        new.authorizations, 
        new.storage_area, 
        COALESCE (1 + 
        (   SELECT 
                max(VERSION)
            FROM 
                person_audit 
            WHERE 
                id = new.id), 1), 
        CURRENT_TIMESTAMP 
    );
RETURN NEW;
END;
$body$ LANGUAGE plpgsql
/
DROP FUNCTION armor (bytea);
--/
CREATE FUNCTION armor (bytea)  RETURNS text
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pg_armor
$body$ LANGUAGE c
/
DROP FUNCTION armor (bytea, text[], text[]);
--/
CREATE FUNCTION armor (bytea, text[], text[])  RETURNS text
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pg_armor
$body$ LANGUAGE c
/
DROP FUNCTION convertfromwiegand (integer);
--/
CREATE FUNCTION convertfromwiegand (p_num integer)  RETURNS integer
  VOLATILE
AS $body$
DECLARE
    v_baseVal varchar(8);
    v_facilityCode varchar(3);
    v_userCode varchar(5);

    v_bitCountdown integer := 24;

    -- All the facility variables we use
    v_facilityBits varchar(8);
    v_fbVal varchar(1);
    v_facilityBitTable varchar array[8]; 
    v_fcPos integer := 1;
    v_facilitySum integer := 0;

    -- And all the user variables
    v_userBits varchar(255);
    v_ubVal varchar(1);
    v_userBitTable varchar array[16];
    v_ucPos integer := 1;
    v_userSum integer := 0;

BEGIN
    v_baseVal := p_num::VARCHAR(8);

    -- We have to be careful about the facility code because it could be 
    -- three digits or less, while the user code will always be five
    -- digits
    v_facilityCode := substring(v_baseVal from 1 for length(v_baseVal) - 5);
    v_userCode := SUBSTRING(v_baseVal from length(v_baseVal) - 4);
    --raise notice '[%] - [%]', v_facilityCode, v_userCode;

    -- Okay, here we go with all our bit-twiddling logic....

    ----------------------------------------------------------------------
    -- Facility Code Logic
    ----------------------------------------------------------------------
    v_facilityBits := v_facilityCode::Integer::bit(8)::varchar;

    for pos in 1..8 loop
        v_fbVal := substring(v_facilityBits from pos for 1);
        if v_fbVal = '1' THEN
            v_facilityBitTable[v_fcPos] = pow(2, v_bitCountdown - 1)::integer::varchar;
        ELSE
            v_facilityBitTable[v_fcPos] = '0';
        end if;

        v_fcPos := v_fcPos + 1;
        v_bitCountdown := v_bitCountdown - 1;
    end loop; 

    for var in array_lower(v_facilityBitTable, 1)..array_upper(v_facilityBitTable, 1) loop
        --raise notice '--> [%]', v_facilityBitTable[var];
        v_facilitySum := v_facilitySum + v_facilityBitTable[var]::INTEGER;
    end loop;

    ----------------------------------------------------------------------
    -- User Code Logic
    ----------------------------------------------------------------------
    v_userBits := v_userCode::INTEGER::bit(16)::VARCHAR;

    for pos in 1..16 loop
        v_ubVal := substring(v_userBits from pos for 1);
        if v_ubVal = '1' THEN
            v_userBitTable[v_ucPos] = pow(2, v_bitCountdown - 1)::integer::varchar;
        ELSE
            v_userBitTable[v_ucPos] = '0';
        end if;

        v_ucPos := v_ucPos + 1;
        v_bitCountdown := v_bitCountdown - 1;
    end loop; 

    for var in array_lower(v_userBitTable, 1)..array_upper(v_userBitTable, 1) loop
        --raise notice '--> [%]', v_userBitTable[var];
        v_userSum := v_userSum + v_userBitTable[var]::INTEGER;
    end loop;

    return (select v_facilitySum + v_userSum);
end;
$body$ LANGUAGE plpgsql
/
DROP FUNCTION converttowiegand (integer);
--/
CREATE FUNCTION converttowiegand (p_num integer)  RETURNS integer
  VOLATILE
AS $body$
DECLARE
    v_baseVal VARCHAR(24) := '';
    v_fc VARCHAR(8) := '';
    v_uc VARCHAR(16) := '';

    v_fNum INTEGER;
    v_uNum INTEGER;

    v_FinalNum varchar(16) := '';
BEGIN
    -- Convert the number passed to us as a binary string
    v_baseVal := CAST(p_num::bit(24)::VARCHAR AS VARCHAR(24));
    -- Okay, we need two parts, the facility code, and the user code
    v_fc := SUBSTRING(v_baseVal from 1 for 8);
    v_uc := SUBSTRING(v_baseVal from 9);
    
    -- Now we're going to convert the bits to numbers
    v_fNum := (v_fc::bit(8))::integer;
    v_uNum := (v_uc::bit(16))::integer;
  
    -- And put it all together    
    v_FinalNum := format('%s%s', v_fNum::varchar, v_uNum::varchar);
  
    RETURN (SELECT v_FinalNum::integer);
END;
$body$ LANGUAGE plpgsql
/
DROP FUNCTION crypt (text, text);
--/
CREATE FUNCTION crypt (text, text)  RETURNS text
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pg_crypt
$body$ LANGUAGE c
/
DROP FUNCTION dearmor (text);
--/
CREATE FUNCTION dearmor (text)  RETURNS bytea
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pg_dearmor
$body$ LANGUAGE c
/
DROP FUNCTION decrypt (bytea, bytea, text);
--/
CREATE FUNCTION decrypt (bytea, bytea, text)  RETURNS bytea
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pg_decrypt
$body$ LANGUAGE c
/
DROP FUNCTION decrypt_iv (bytea, bytea, bytea, text);
--/
CREATE FUNCTION decrypt_iv (bytea, bytea, bytea, text)  RETURNS bytea
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pg_decrypt_iv
$body$ LANGUAGE c
/
DROP FUNCTION digest (bytea, text);
--/
CREATE FUNCTION digest (bytea, text)  RETURNS bytea
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pg_digest
$body$ LANGUAGE c
/
DROP FUNCTION digest (text, text);
--/
CREATE FUNCTION digest (text, text)  RETURNS bytea
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pg_digest
$body$ LANGUAGE c
/
DROP FUNCTION encrypt (bytea, bytea, text);
--/
CREATE FUNCTION encrypt (bytea, bytea, text)  RETURNS bytea
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pg_encrypt
$body$ LANGUAGE c
/
DROP FUNCTION encrypt_iv (bytea, bytea, bytea, text);
--/
CREATE FUNCTION encrypt_iv (bytea, bytea, bytea, text)  RETURNS bytea
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pg_encrypt_iv
$body$ LANGUAGE c
/
DROP FUNCTION gen_random_bytes (integer);
--/
CREATE FUNCTION gen_random_bytes (integer)  RETURNS bytea
  VOLATILE
  RETURNS NULL ON NULL INPUT
AS $body$
pg_random_bytes
$body$ LANGUAGE c
/
DROP FUNCTION gen_random_uuid ();
--/
CREATE FUNCTION gen_random_uuid ()  RETURNS uuid
  VOLATILE
AS $body$
pg_random_uuid
$body$ LANGUAGE c
/
DROP FUNCTION gen_salt (text);
--/
CREATE FUNCTION gen_salt (text)  RETURNS text
  VOLATILE
  RETURNS NULL ON NULL INPUT
AS $body$
pg_gen_salt
$body$ LANGUAGE c
/
DROP FUNCTION gen_salt (text, integer);
--/
CREATE FUNCTION gen_salt (text, integer)  RETURNS text
  VOLATILE
  RETURNS NULL ON NULL INPUT
AS $body$
pg_gen_salt_rounds
$body$ LANGUAGE c
/
DROP FUNCTION hmac (bytea, bytea, text);
--/
CREATE FUNCTION hmac (bytea, bytea, text)  RETURNS bytea
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pg_hmac
$body$ LANGUAGE c
/
DROP FUNCTION hmac (text, text, text);
--/
CREATE FUNCTION hmac (text, text, text)  RETURNS bytea
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pg_hmac
$body$ LANGUAGE c
/
DROP FUNCTION pgp_armor_headers (text);
--/
CREATE FUNCTION pgp_armor_headers (text, OUT key text, OUT value text)  RETURNS SETOF record
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_armor_headers
$body$ LANGUAGE c
/
DROP FUNCTION pgp_key_id (bytea);
--/
CREATE FUNCTION pgp_key_id (bytea)  RETURNS text
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_key_id_w
$body$ LANGUAGE c
/
DROP FUNCTION pgp_pub_decrypt (bytea, bytea);
--/
CREATE FUNCTION pgp_pub_decrypt (bytea, bytea)  RETURNS text
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_pub_decrypt_text
$body$ LANGUAGE c
/
DROP FUNCTION pgp_pub_decrypt (bytea, bytea, text);
--/
CREATE FUNCTION pgp_pub_decrypt (bytea, bytea, text)  RETURNS text
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_pub_decrypt_text
$body$ LANGUAGE c
/
DROP FUNCTION pgp_pub_decrypt (bytea, bytea, text, text);
--/
CREATE FUNCTION pgp_pub_decrypt (bytea, bytea, text, text)  RETURNS text
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_pub_decrypt_text
$body$ LANGUAGE c
/
DROP FUNCTION pgp_pub_decrypt_bytea (bytea, bytea);
--/
CREATE FUNCTION pgp_pub_decrypt_bytea (bytea, bytea)  RETURNS bytea
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_pub_decrypt_bytea
$body$ LANGUAGE c
/
DROP FUNCTION pgp_pub_decrypt_bytea (bytea, bytea, text);
--/
CREATE FUNCTION pgp_pub_decrypt_bytea (bytea, bytea, text)  RETURNS bytea
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_pub_decrypt_bytea
$body$ LANGUAGE c
/
DROP FUNCTION pgp_pub_decrypt_bytea (bytea, bytea, text, text);
--/
CREATE FUNCTION pgp_pub_decrypt_bytea (bytea, bytea, text, text)  RETURNS bytea
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_pub_decrypt_bytea
$body$ LANGUAGE c
/
DROP FUNCTION pgp_pub_encrypt (text, bytea);
--/
CREATE FUNCTION pgp_pub_encrypt (text, bytea)  RETURNS bytea
  VOLATILE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_pub_encrypt_text
$body$ LANGUAGE c
/
DROP FUNCTION pgp_pub_encrypt (text, bytea, text);
--/
CREATE FUNCTION pgp_pub_encrypt (text, bytea, text)  RETURNS bytea
  VOLATILE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_pub_encrypt_text
$body$ LANGUAGE c
/
DROP FUNCTION pgp_pub_encrypt_bytea (bytea, bytea);
--/
CREATE FUNCTION pgp_pub_encrypt_bytea (bytea, bytea)  RETURNS bytea
  VOLATILE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_pub_encrypt_bytea
$body$ LANGUAGE c
/
DROP FUNCTION pgp_pub_encrypt_bytea (bytea, bytea, text);
--/
CREATE FUNCTION pgp_pub_encrypt_bytea (bytea, bytea, text)  RETURNS bytea
  VOLATILE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_pub_encrypt_bytea
$body$ LANGUAGE c
/
DROP FUNCTION pgp_sym_decrypt (bytea, text);
--/
CREATE FUNCTION pgp_sym_decrypt (bytea, text)  RETURNS text
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_sym_decrypt_text
$body$ LANGUAGE c
/
DROP FUNCTION pgp_sym_decrypt (bytea, text, text);
--/
CREATE FUNCTION pgp_sym_decrypt (bytea, text, text)  RETURNS text
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_sym_decrypt_text
$body$ LANGUAGE c
/
DROP FUNCTION pgp_sym_decrypt_bytea (bytea, text);
--/
CREATE FUNCTION pgp_sym_decrypt_bytea (bytea, text)  RETURNS bytea
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_sym_decrypt_bytea
$body$ LANGUAGE c
/
DROP FUNCTION pgp_sym_decrypt_bytea (bytea, text, text);
--/
CREATE FUNCTION pgp_sym_decrypt_bytea (bytea, text, text)  RETURNS bytea
  IMMUTABLE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_sym_decrypt_bytea
$body$ LANGUAGE c
/
DROP FUNCTION pgp_sym_encrypt (text, text);
--/
CREATE FUNCTION pgp_sym_encrypt (text, text)  RETURNS bytea
  VOLATILE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_sym_encrypt_text
$body$ LANGUAGE c
/
DROP FUNCTION pgp_sym_encrypt (text, text, text);
--/
CREATE FUNCTION pgp_sym_encrypt (text, text, text)  RETURNS bytea
  VOLATILE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_sym_encrypt_text
$body$ LANGUAGE c
/
DROP FUNCTION pgp_sym_encrypt_bytea (bytea, text);
--/
CREATE FUNCTION pgp_sym_encrypt_bytea (bytea, text)  RETURNS bytea
  VOLATILE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_sym_encrypt_bytea
$body$ LANGUAGE c
/
DROP FUNCTION pgp_sym_encrypt_bytea (bytea, text, text);
--/
CREATE FUNCTION pgp_sym_encrypt_bytea (bytea, text, text)  RETURNS bytea
  VOLATILE
  RETURNS NULL ON NULL INPUT
AS $body$
pgp_sym_encrypt_bytea
$body$ LANGUAGE c
/
DROP TRIGGER trg_person ON person CASCADE;
--/
CREATE TRIGGER trg_person
  AFTER INSERT OR UPDATE ON person
  FOR EACH ROW
EXECUTE FUNCTION add_person_audit_record()
/
