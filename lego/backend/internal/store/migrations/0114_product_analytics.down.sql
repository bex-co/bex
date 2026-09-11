DROP TRIGGER product_app_delete ON apps;
DROP FUNCTION bex_product_app_delete();
DROP TRIGGER product_domain_event ON domains;
DROP FUNCTION bex_product_domain_event();
DROP TRIGGER product_deploy_event ON deploys;
DROP FUNCTION bex_product_deploy_event();
DROP TABLE product_domain_observations, product_hosting_daily, product_analytics_audiences, product_activity_events, product_analytics_collection;
DROP TRIGGER product_workspace_event ON tenants;
DROP FUNCTION bex_product_workspace_event();
