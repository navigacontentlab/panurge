# Panurge

>[...] Panurge, without another word, threw his sheep, crying and bleating, into the sea. All the other sheep, crying and bleating in the same intonation, started to throw themselves in the sea after it, all in a line. The herd was such that once one jumped, so jumped its companions. It was not possible to stop them, as you know, with sheep, it's natural to always follow the first one, wherever it may go.
>— Francois Rabelais, Quart Livre, chapter VIII

Common libraries and utility functions for our Go applications.

## Example usage of StandardApp

```
	opts := options{
		Port:         8081,
		InternalPort: 8090,
	}

	err := copperhead.Configure(&opts,
		copperhead.WithEnvironment(map[string]string{
			"Port":                   "PORT",
			"InternalPort":           "INTERNAL_PORT",
			"ImasURL":                "IMAS_URL",
		}),
		copperhead.Require(
			"ImasURL",
		),
	)
	if err != nil {
		return fmt.Errorf("failed to read configuration: %v", err)
	}

	sess, err := session.NewSession()
	if err != nil {
		return errors.Errorf(
			"failed to create AWS SDK session: %w", err)
	}

	service, err := sendmail.NewEmailService(sendmail.EmailServiceOptions{
		Sender: sesv2.New(sess),
	})

	app, err := panurge.NewStandardApp(logger,
		panurge.WithAppPorts(
			opts.Port, opts.InternalPort),
	    panurge.WithImasURL(opts.ImasURL),
		panurge.WithAppService(
			rpc.EmailPathPrefix,
			func(hooks *twirp.ServerHooks) http.Handler {
				return rpc.NewEmailServer(service, hooks)
			},
		),
	)
	if err != nil {
		return err
	}

	return app.ListenAndServe()
```
