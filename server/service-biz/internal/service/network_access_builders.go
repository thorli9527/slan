package service

import "github.com/slan/service-biz/internal/model"

func publicMappingViews(items []model.PublicMapping) []PublicMappingView {
	views := make([]PublicMappingView, 0, len(items))
	for _, item := range items {
		views = append(views, publicMappingView(item))
	}
	return views
}

func securityGroupViews(items []model.SecurityGroup) []SecurityGroupView {
	views := make([]SecurityGroupView, 0, len(items))
	for _, item := range items {
		views = append(views, securityGroupView(item))
	}
	return views
}

func securityRuleViews(items []model.SecurityRule) []SecurityRuleView {
	views := make([]SecurityRuleView, 0, len(items))
	for _, item := range items {
		views = append(views, securityRuleView(item))
	}
	return views
}
