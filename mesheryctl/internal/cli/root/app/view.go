package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/gofrs/uuid"
	"github.com/layer5io/meshery/internal/sql"
	"github.com/layer5io/meshery/mesheryctl/internal/cli/root/config"
	"github.com/layer5io/meshery/mesheryctl/internal/cli/root/constants"
	"github.com/layer5io/meshery/mesheryctl/pkg/utils"
	"github.com/layer5io/meshery/models"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type appStruct struct {
	Name            string
	ID              *uuid.UUID
	ApplicationFile string
	UserID          *string
	Location        sql.Map
	UpdatedAt       *time.Time
	CreatedAt       *time.Time
}

var (
	viewAllFlag   bool
	outFormatFlag string
)

var viewCmd = &cobra.Command{
	Use:   "view <application name>",
	Short: "Display application(s)",
	Long:  `Displays the contents of a specific application based on name or id`,
	Example: `
	View applictaions with name
	mesheryctl app view <app-name>

	View applications with id
	mesheryctl app view <app-id>

	View all applications
	mesheryctl app view --all
	`,
	Args: cobra.MinimumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		mctlCfg, err := config.GetMesheryCtl(viper.GetViper())
		if err != nil {
			return errors.Wrap(err, "error processing config")
		}
		// set default tokenpath for app offboard command.
		if tokenPath == "" {
			tokenPath = constants.GetCurrentAuthToken()
		}
		application := ""
		isID := false
		applicationID := ""
		// if application name/id available
		if len(args) > 0 {
			if viewAllFlag {
				return errors.New("-a cannot be used when [application-name|application-id] is specified")
			}
			applicationID = args[0]
			// check if the application argument is a valid uuid v4 string
			isID, err = regexp.MatchString("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}$", applicationID)
			if err != nil {
				return err
			}
		}
		var req *http.Request
		url := mctlCfg.GetBaseMesheryURL()
		var response *models.ApplicationsAPIResponse
		// Merge args to get app-name
		application = strings.Join(args, "%20")
		if len(application) == 0 {
			if viewAllFlag {
				url += "/api/experimental/application?page_size=10000"
			} else {
				return errors.New("[application-name|application-id] not specified, use -a to view all applications")
			}
		} else if isID {
			// if application is a valid uuid, then directly fetch the application
			url += "/api/experimental/application/" + applicationID
		} else {
			// else search application by name
			url += "/api/experimental/application?search=" + application
		}

		client := &http.Client{}
		req, err = http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}

		err = utils.AddAuthDetails(req, tokenPath)
		if err != nil {
			return err
		}

		res, err := client.Do(req)
		if err != nil {
			return err
		}
		if res.StatusCode != 200 {
			// failsafe for the case when a valid uuid v4 is not an id of any application (bad api call)
			return errors.Errorf("Response Status Code %d, possible invalid ID", res.StatusCode)
		}

		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}

		var dat map[string]interface{}
		if err = json.Unmarshal(body, &dat); err != nil {
			return errors.Wrap(err, "failed to unmarshal response body")
		}
		if err = json.Unmarshal(body, &response); err != nil {
			return errors.Wrap(err, "failed to unmarshal response body")
		}
		if isID {
			if body, err = json.MarshalIndent(dat, "", "  "); err != nil {
				return err
			}
		} else if viewAllFlag {
			// only keep the application key from the response when viewing all the applications
			if body, err = json.MarshalIndent(map[string]interface{}{"applications": dat["applications"]}, "", "  "); err != nil {
				return err
			}
		} else {
			var a appStruct
			if response.TotalCount == 0 {
				return errors.New("application does not exit. Please get an app name and try again. Use `mesheryctl app list` to see a list of applications")
			}
			for _, app := range response.Applications {
				if response.Applications == nil {
					return errors.New("application name not provide. Please get an app name and try again. Use `mesheryctl app list` to see a list of applications")
				}
				if app.Name == application {
					a = appStruct{
						Name:            app.Name,
						ID:              app.ID,
						ApplicationFile: app.ApplicationFile,
						UserID:          app.UserID,
						Location:        app.Location,
						UpdatedAt:       app.UpdatedAt,
						CreatedAt:       app.CreatedAt,
					}
					body, err = json.MarshalIndent(&a, "", "  ")
					if err != nil {
						return err
					}
					fmt.Printf("Name: %v\n", a.Name)
					fmt.Printf("ID: %s\n", a.ID.String())
					fmt.Printf("ApplicationFile: %v\n", a.ApplicationFile)
					fmt.Printf("UpdatedAt: %s\n", a.UpdatedAt.String())
					fmt.Printf("CreatedAt: %s\n", a.CreatedAt.String())
					fmt.Printf("UserID: %v\n", a.UserID)
					fmt.Printf("Location: %v\n", a.Location)
					fmt.Println("#####################")
					continue
				}
			}
		}

		if outFormatFlag == "yaml" {
			if body, err = yaml.JSONToYAML(body); err != nil {
				return errors.Wrap(err, "failed to convert json to yaml")
			}
		} else if outFormatFlag != "json" {
			return errors.New("output-format choice invalid, use [json|yaml]")
		}
		return nil
	},
}

func init() {
	viewCmd.Flags().BoolVarP(&viewAllFlag, "all", "a", false, "(optional) view all applications available")
	viewCmd.Flags().StringVarP(&outFormatFlag, "output-format", "o", "yaml", "(optional) format to display in [json|yaml]")
}