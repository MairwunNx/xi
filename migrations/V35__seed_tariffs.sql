TRUNCATE TABLE xi_tariffs CASCADE;

-- Bronze тариф (Free)
INSERT INTO xi_tariffs (
    key,
    display_name,
    requests_per_day,
    requests_per_month,
    tokens_per_day,
    tokens_per_month,
    spending_daily_limit,
    spending_monthly_limit,
    price
) VALUES (
    'bronze',
    'Bronze',
    50,
    1000,
    100000,
    2000000,
    0.10,
    1.50,
    0
);

-- Silver тариф (Plus)
INSERT INTO xi_tariffs (
    key,
    display_name,
    requests_per_day,
    requests_per_month,
    tokens_per_day,
    tokens_per_month,
    spending_daily_limit,
    spending_monthly_limit,
    price
) VALUES (
    'silver',
    'Silver',
    150,
    3000,
    150000,
    4000000,
    0.50,
    12.00,
    999
);

-- Gold тариф (Pro)
INSERT INTO xi_tariffs (
    key,
    display_name,
    requests_per_day,
    requests_per_month,
    tokens_per_day,
    tokens_per_month,
    spending_daily_limit,
    spending_monthly_limit,
    price
) VALUES (
    'gold',
    'Gold',
    200,
    4000,
    200000,
    4250000,
    0.70,
    17.00,
    1499
);
