package app

import servicepkg "github.com/slan/service-biz/internal/service"

func newDownloadServices(deps UseCaseDependencies) DownloadServices {
	repos := deps.downloadRepositories()
	return DownloadServices{
		ClientDelivery: servicepkg.ClientDownloadService{
			Catalog: repos.Catalog,
		},
	}
}
