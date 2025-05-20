package dumper

import (
	"context"
	"errors"

	"go.chromium.org/infra/unifiedfleet/app/controller"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

// Only MachineLSEs for chromeOS
var machineLSEsExportNamespaces = []string{util.OSNamespace}

func flushMachineLSEsWithoutRealm(ctx context.Context) error {
	var errs []error
	for _, ns := range machineLSEsExportNamespaces {
		datastoreNamespace := util.ClientToDatastoreNamespace[ns]
		ctx, err := util.SetupDatastoreNamespace(ctx, datastoreNamespace)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		err = controller.DeleteMachineLSEsWithoutRealm(ctx)
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}
	return errors.Join(errs...)
}
