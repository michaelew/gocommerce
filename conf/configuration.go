package conf

import (
	"net/url"
	"os"
	"strconv"
	"strings"

	"bufio"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// Configuration holds all the confiruation for authlify
type Configuration struct {
	SiteURL string `mapstructure:"site_url" json:"site_url"`

	JWT struct {
		Secret         string `mapstructure:"secret" json:"secret"`
		AdminGroupName string `mapstructure:"admin_group_name" json:"admin_group_name"`
	} `mapstructure:"jwt" json:"jwt"`

	DB struct {
		Driver      string `mapstructure:"driver" json:"driver"`
		ConnURL     string `mapstructure:"url" json:"url"`
		Namespace   string `mapstructure:"namespace" json:"namespace"`
		Automigrate bool   `mapstructure:"automigrate" json:"automigrate"`
	} `mapstructure:"db" json:"db"`

	API struct {
		Host string `mapstructure:"host" json:"host"`
		Port int    `mapstructure:"port" json:"port"`
	} `mapstructure:"api" json:"api"`
	LogConf struct {
		Level string `mapstructure:"level"`
		File  string `mapstructure:"file"`
	} `mapstructure:"log_conf"`
	Mailer struct {
		Host       string `mapstructure:"host" json:"host"`
		Port       int    `mapstructure:"port" json:"port"`
		User       string `mapstructure:"user" json:"user"`
		Pass       string `mapstructure:"pass" json:"pass"`
		AdminEmail string `mapstructure:"admin_email" json:"admin_email"`
		Subjects   struct {
			OrderConfirmation string `mapstructure:"order_confirmation" json:"order_confirmation"`
			OrderReceived     string `mapstructure:"order_received" json:"order_received"`
		} `mapstructure:"subjects" json:"subjects"`
		Templates struct {
			OrderConfirmation string `mapstructure:"order_confirmation" json:"order_confirmation"`
			OrderReceived     string `mapstructure:"order_received" json:"order_received"`
		} `mapstructure:"templates" json:"templates"`
	} `mapstructure:"mailer" json:"mailer"`

	Payment struct {
		Stripe struct {
			SecretKey string `mapstructure:"secret_key" json:"secret_key"`
		} `mapstructure:"stripe" json:"stripe"`
		Paypal struct {
			ClientID string `mapstructure:"client_id" json:"client_id"`
			Secret   string `mapstructure:"secret" json:"secret"`
			Env      string `mapstructure:"env" json:"env"`
		} `mapstructure:"paypal" json:"paypal"`
	} `mapstructure:"payment" json:"payment"`

	Downloads struct {
		Provider     string `mapstructure:"provider" json:"provider"`
		NetlifyToken string `mapstructure:"netlify_token" json:"netlify_token"`
	} `mapstructure:"downloads" json:"downloads"`

	Coupons struct {
		URL      string `mapstructure:"url" json:"url"`
		User     string `mapstructure:"user" json:"user"`
		Password string `mapstructure:"password" json:"password"`
	} `mapstructure:"coupons" json:"coupons"`

	Webhooks struct {
		Order   string `mapstructure:"order" json:"order"`
		Payment string `mapstructure:"payment" json:"payment"`
		Update  string `mapstructure:"update" json:"update"`
		Refund  string `mapstructure:"refund" json:"refund"`

		Secret string `mapstructure:"secret" json:"secret"`
	} `mapstructure:"webhooks" json:"webhooks"`
}

// Load will construct the config from the file `config.json`
func Load(configFile string) (*Configuration, error) {
	viper.SetConfigType("json")

	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath("./")                 // ./config.[json | toml]
		viper.AddConfigPath("$HOME/.gocommerce/") // ~/.gocommerce/config.[json | toml] // Keep the configuration backwards compatible
	}

	viper.SetEnvPrefix("GOCOMMERCE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "reading configuration from files")
	}

	config := new(Configuration)
	if err := viper.Unmarshal(config); err != nil {
		return nil, errors.Wrap(err, "unmarshaling configuration")
	}

	config, err := populateConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "populate config")
	}

	if err := configureLogging(config); err != nil {
		return nil, errors.Wrap(err, "configure logging")
	}

	if config.JWT.AdminGroupName == "" {
		config.JWT.AdminGroupName = "admin"
	}

	return validateConfig(config)
}

func configureLogging(config *Configuration) error {
	// always use the full timestamp
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:    true,
		DisableTimestamp: false,
	})

	// use a file if you want
	if config.LogConf.File != "" {
		f, errOpen := os.OpenFile(config.LogConf.File, os.O_RDWR|os.O_APPEND, 0660)
		if errOpen != nil {
			return errOpen
		}
		logrus.SetOutput(bufio.NewWriter(f))
		logrus.Infof("Set output file to %s", config.LogConf.File)
	}

	if config.LogConf.Level != "" {
		level, err := logrus.ParseLevel(strings.ToUpper(config.LogConf.Level))
		if err != nil {
			return err
		}
		logrus.SetLevel(level)
		logrus.Debug("Set log level to: " + logrus.GetLevel().String())
	}

	return nil
}

func validateConfig(config *Configuration) (*Configuration, error) {
	if config.DB.ConnURL == "" && os.Getenv("DATABASE_URL") != "" {
		config.DB.ConnURL = os.Getenv("DATABASE_URL")
	}

	if config.DB.Driver == "" && config.DB.ConnURL != "" {
		u, err := url.Parse(config.DB.ConnURL)
		if err != nil {
			return nil, errors.Wrap(err, "parsing db connection url")
		}
		config.DB.Driver = u.Scheme
	}

	if config.API.Port == 0 && os.Getenv("PORT") != "" {
		port, err := strconv.Atoi(os.Getenv("PORT"))
		if err != nil {
			return nil, errors.Wrap(err, "formatting PORT into int")
		}

		config.API.Port = port
	}

	if config.API.Port == 0 && config.API.Host == "" {
		config.API.Port = 8080
	}

	return config, nil
}
