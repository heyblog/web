package dataimport

func validatePlan(plan Plan) error {
	sites, err := validatePlanSites(plan)
	if err != nil {
		return err
	}
	if err := validatePlanLocations(plan, sites.byID); err != nil {
		return err
	}
	if err := validatePlanTags(plan, sites.byID); err != nil {
		return err
	}
	if err := validatePlanComponents(plan, sites.byID); err != nil {
		return err
	}
	if err := validatePlanOrigins(plan, sites.byID); err != nil {
		return err
	}
	return validatePlanFriendLinks(plan, sites)
}
