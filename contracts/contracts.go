// Package contracts is the stable import path for the types an application and a plugin
// implement, following Goravel's package layout (`contracts/`, `facades/`).
//
// The types are aliases of the panel package, so a value of one is a value of the other:
// existing code keeps working and `contracts.Plugin` and `mks.Plugin` are the same type.
//
// The facade returns the concrete *mks.Panel rather than an interface on purpose. Plugins
// and commands use the whole panel API (Register, Use, hooks, routes), so an interface
// would have to mirror the entire type without giving applications anything to substitute.
package contracts

import mks "github.com/MiranSalehi/mks"

type (
	// Panel is the panel itself; *mks.Panel satisfies it.
	Panel = mks.Panel
	// Options configures the panel before it is built.
	Options = mks.Options
	// AuthOptions configures the panel's login.
	AuthOptions = mks.AuthOptions
	// TenancyOptions turns on multi-tenancy.
	TenancyOptions = mks.TenancyOptions
	// User is the application's user model as the panel sees it.
	User = mks.User
	// UserFactory makes an empty user, one per request.
	UserFactory = mks.UserFactory
	// Resource is a resource definition (see mks.Define).
	Resource = mks.Resource
	// Config is a resource's configuration.
	Config = mks.Config
	// Plugin is a bundle of panel features installed with Panel.Use.
	Plugin = mks.Plugin
	// RoutesPlugin is a plugin with its own API routes.
	RoutesPlugin = mks.RoutesPlugin
	// Gate is the panel-wide permission check.
	Gate = mks.Gate
	// HookContext is a request's user and panel, for hooks and handlers.
	HookContext = mks.HookContext
	// OptionsHook changes the options before the panel is built (mks.OptionsBinding).
	OptionsHook = mks.OptionsHook
	// ConfigureHook registers resources and plugins after the panel is built
	// (mks.ConfigureBinding).
	ConfigureHook = mks.ConfigureHook
)
