package granted

import (
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"path"

	"github.com/common-fate/clio"
	"github.com/common-fate/clio/clierr"
	"github.com/common-fate/granted/pkg/assume"
	"github.com/common-fate/granted/pkg/browser"
	"github.com/common-fate/granted/pkg/cfaws"
	"github.com/common-fate/granted/pkg/config"
	"github.com/common-fate/granted/pkg/console"
	"github.com/common-fate/granted/pkg/forkprocess"
	"github.com/common-fate/granted/pkg/launcher"
	"github.com/urfave/cli/v2"
)

var ConsoleCommand = cli.Command{
	Name:  "console",
	Usage: "Generate an AWS console URL using credentials in the environment",
	Flags: []cli.Flag{

		&cli.StringFlag{Name: "service"},
		&cli.StringFlag{Name: "region", EnvVars: []string{"AWS_REGION"}},
		&cli.StringFlag{Name: "destination", Usage: "The destination URL for the console"},
		&cli.BoolFlag{Name: "url", Usage: "Return the URL to stdout instead of launching the browser"},
		&cli.BoolFlag{Name: "firefox", Usage: "Generate the Firefox container URL"},
		&cli.StringFlag{Name: "color", Usage: "When the firefox flag is true, this specifies the color of the container tab"},
		&cli.StringFlag{Name: "icon", Usage: "When firefox flag is true, this specifies the icon of the container tab"},
		&cli.StringFlag{Name: "container-name", Usage: "When firefox flag is true, this specifies the name of the container of the container tab.", Value: "aws"},
	},
	Action: func(c *cli.Context) error {
		ctx := c.Context
		credentials := cfaws.GetEnvCredentials(ctx)
		con := console.AWS{
			Service:     c.String("service"),
			Region:      c.String("region"),
			Destination: c.String("destination"),
		}

		consoleURL, err := con.URL(credentials)
		if err != nil {
			return err
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if c.Bool("firefox") || cfg.DefaultBrowser == browser.FirefoxKey || cfg.DefaultBrowser == browser.FirefoxStdoutKey {
			// transform the URL into the Firefox Tab Container format.
			consoleURL = fmt.Sprintf("ext+granted-containers:name=%s&url=%s&color=%s&icon=%s", c.String("container-name"), url.QueryEscape(consoleURL), c.String("color"), c.String("icon"))
		}

		justPrintURL := c.Bool("url") || cfg.DefaultBrowser == browser.StdoutKey || cfg.DefaultBrowser == browser.FirefoxStdoutKey
		if justPrintURL {
			// return the url via stdout through the CLI wrapper script and return early.
			fmt.Print(consoleURL)
			return nil
		}

		grantedFolder, err := config.GrantedConfigFolder()
		if err != nil {
			return err
		}

		var l assume.Launcher
		if cfg.CustomBrowserPath == "" && cfg.DefaultBrowser != "" {
			l = launcher.Open{}
		} else if cfg.CustomBrowserPath == "" {
			return errors.New("default browser not configured. run `granted browser set` to configure")
		} else {
			switch cfg.DefaultBrowser {
			case browser.ChromeKey:
				l = launcher.ChromeProfile{
					ExecutablePath: cfg.CustomBrowserPath,
					UserDataPath:   path.Join(grantedFolder, "chromium-profiles", "1"), // held over for backwards compatibility, "1" indicates Chrome profiles
				}
			case browser.BraveKey:
				l = launcher.ChromeProfile{
					ExecutablePath: cfg.CustomBrowserPath,
					UserDataPath:   path.Join(grantedFolder, "chromium-profiles", "2"), // held over for backwards compatibility, "2" indicates Brave profiles
				}
			case browser.EdgeKey:
				l = launcher.ChromeProfile{
					ExecutablePath: cfg.CustomBrowserPath,
					UserDataPath:   path.Join(grantedFolder, "chromium-profiles", "3"), // held over for backwards compatibility, "3" indicates Edge profiles
				}
			case browser.ChromiumKey:
				l = launcher.ChromeProfile{
					ExecutablePath: cfg.CustomBrowserPath,
					UserDataPath:   path.Join(grantedFolder, "chromium-profiles", "4"), // held over for backwards compatibility, "4" indicates Chromium profiles
				}
			case browser.FirefoxKey:
				l = launcher.Firefox{
					ExecutablePath: cfg.CustomBrowserPath,
				}
			case browser.SafariKey:
				l = launcher.Safari{}
			default:
				l = launcher.Open{}
			}
		}
		// now build the actual command to run - e.g. 'firefox --new-tab <URL>'
		args := l.LaunchCommand(consoleURL, con.Profile)

		var startErr error
		if l.UseForkProcess() {
			clio.Debugf("running command using forkprocess: %s", args)
			cmd, err := forkprocess.New(args...)
			if err != nil {
				return err
			}
			startErr = cmd.Start()
		} else {
			clio.Debugf("running command without forkprocess: %s", args)
			cmd := exec.Command(args[0], args[1:]...)
			startErr = cmd.Start()
		}

		if startErr != nil {
			return clierr.New(fmt.Sprintf("Granted was unable to open a browser session automatically due to the following error: %s", err.Error()),
				// allow them to try open the url manually
				clierr.Info("You can open the browser session manually using the following url:"),
				clierr.Info(consoleURL),
			)
		}
		return nil
	},
}
