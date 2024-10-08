// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

// Package application provides the domain types for an application.
// The application domain is the primary domain for creation and handling of an
// application.
//
// The charm is the stored representation of the application. The application
// is the instance manifest of the charm and the unit is the running instance
// of the application.
//
// Charm types are stored in the application/charm package, to ensure that the
// charm is handled correctly and that the charm is correctly represented in
// the domain.
//
// A resource is a stored representation of a charm's resource. There are 3
// types of resources after deploy: an application resource, a unit resource
// and a repository resource. Once deployed, resources can be refreshed
// independently from a charm. Thus they will have relations to applications
// but not a specific charm revision.
//
// Resource types are stored in the application/resource package, to ensure that the
// resources is handled correctly and that the resource is correctly represented in
// the domain.
package application
