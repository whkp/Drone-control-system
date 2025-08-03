package main

import (
	"flag"
	"log"

	"drone-control-system/pkg/database"

	"github.com/spf13/viper"
)

func main() {
	var (
		configPath = flag.String("config", "./configs/config.yaml", "配置文件路径")
		action     = flag.String("action", "migrate", "操作类型: create, migrate, seed, reset, drop, health")
		force      = flag.Bool("force", false, "强制执行操作")
	)
	flag.Parse()

	// 加载配置
	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 创建数据库配置
	mysqlConfig := database.Config{
		Host:            config.GetString("database.mysql.host"),
		Port:            config.GetInt("database.mysql.port"),
		User:            config.GetString("database.mysql.user"),
		Password:        config.GetString("database.mysql.password"),
		DBName:          config.GetString("database.mysql.dbname"),
		Charset:         config.GetString("database.mysql.charset"),
		ParseTime:       config.GetBool("database.mysql.parse_time"),
		Loc:             config.GetString("database.mysql.loc"),
		MaxOpenConns:    config.GetInt("database.mysql.max_open_conns"),
		MaxIdleConns:    config.GetInt("database.mysql.max_idle_conns"),
		ConnMaxLifetime: config.GetDuration("database.mysql.conn_max_lifetime"),
		ConnMaxIdleTime: config.GetDuration("database.mysql.conn_max_idle_time"),
		LogLevel:        config.GetString("database.mysql.log_level"),
	}

	// 如果配置为空，使用默认配置
	if mysqlConfig.Host == "" {
		mysqlConfig = database.DefaultConfig()
	}

	switch *action {
	case "create":
		if err := database.CreateDatabase(mysqlConfig); err != nil {
			log.Fatalf("创建数据库失败: %v", err)
		}
		log.Println("数据库创建成功!")

	case "migrate":
		db, err := database.NewMySQLConnection(mysqlConfig)
		if err != nil {
			log.Fatalf("连接数据库失败: %v", err)
		}

		if err := database.Migrate(db); err != nil {
			log.Fatalf("数据库迁移失败: %v", err)
		}
		log.Println("数据库迁移完成!")

	case "seed":
		db, err := database.NewMySQLConnection(mysqlConfig)
		if err != nil {
			log.Fatalf("连接数据库失败: %v", err)
		}

		if err := database.SeedData(db); err != nil {
			log.Fatalf("种子数据创建失败: %v", err)
		}
		log.Println("种子数据创建完成!")

	case "health":
		db, err := database.NewMySQLConnection(mysqlConfig)
		if err != nil {
			log.Fatalf("连接数据库失败: %v", err)
		}

		health := database.HealthCheck(db)
		log.Printf("数据库健康检查结果: %+v", health)

	case "reset":
		if !*force {
			log.Fatal("重置数据库需要使用 -force 参数")
		}

		// 删除并重新创建数据库
		if err := database.DropDatabase(mysqlConfig); err != nil {
			log.Printf("删除数据库警告: %v", err)
		}

		if err := database.CreateDatabase(mysqlConfig); err != nil {
			log.Fatalf("创建数据库失败: %v", err)
		}

		// 重新连接并迁移
		db, err := database.NewMySQLConnection(mysqlConfig)
		if err != nil {
			log.Fatalf("重新连接数据库失败: %v", err)
		}

		if err := database.Migrate(db); err != nil {
			log.Fatalf("数据库迁移失败: %v", err)
		}

		if err := database.SeedData(db); err != nil {
			log.Fatalf("种子数据创建失败: %v", err)
		}

		log.Println("数据库重置完成!")

	case "drop":
		if !*force {
			log.Fatal("删除数据库需要使用 -force 参数")
		}

		if err := database.DropDatabase(mysqlConfig); err != nil {
			log.Fatalf("删除数据库失败: %v", err)
		}
		log.Println("数据库删除完成!")

	default:
		log.Fatalf("未知操作: %s", *action)
	}
}

func loadConfig(configPath string) (*viper.Viper, error) {
	config := viper.New()
	config.SetConfigFile(configPath)
	config.SetConfigType("yaml")

	// 设置默认值
	config.SetDefault("database.mysql.host", "localhost")
	config.SetDefault("database.mysql.port", 3306)
	config.SetDefault("database.mysql.user", "root")
	config.SetDefault("database.mysql.password", "password")
	config.SetDefault("database.mysql.dbname", "drone_control")
	config.SetDefault("database.mysql.charset", "utf8mb4")
	config.SetDefault("database.mysql.parse_time", true)
	config.SetDefault("database.mysql.loc", "Local")
	config.SetDefault("database.mysql.max_open_conns", 100)
	config.SetDefault("database.mysql.max_idle_conns", 10)
	config.SetDefault("database.mysql.conn_max_lifetime", "1h")
	config.SetDefault("database.mysql.conn_max_idle_time", "30m")
	config.SetDefault("database.mysql.log_level", "info")

	if err := config.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// 配置文件不存在，使用默认值
			log.Printf("配置文件不存在，使用默认配置: %s", configPath)
		} else {
			return nil, err
		}
	}

	return config, nil
}
