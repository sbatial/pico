ALTER table app_users ADD stripe_customer_id varchar(256) NOT NULL DEFAULT '';
ALTER table app_users ADD pro_expires_at timestamp without time zone;
