// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"database/sql"
	"testing"

	"github.com/juju/tc"

	corenetwork "github.com/juju/juju/core/network"
	"github.com/juju/juju/domain/network"
)

type netConfigSuite struct {
	linkLayerBaseSuite
}

func TestNetConfigSuite(t *testing.T) {
	tc.Run(t, &netConfigSuite{})
}

func (s *netConfigSuite) TestSetMachineNetConfig(c *tc.C) {
	db := s.DB()

	// Arrange
	nodeUUID := "net-node-uuid"
	devName := "eth0"
	subnetUUID := "subnet-uuid"

	ctx := c.Context()

	_, err := db.ExecContext(ctx, "INSERT INTO net_node (uuid) VALUES (?)", nodeUUID)
	c.Assert(err, tc.ErrorIsNil)

	_, err = db.ExecContext(ctx, "INSERT INTO subnet (uuid, cidr, space_uuid) VALUES (?, ?, ?)",
		subnetUUID, "192.168.0.0/24", corenetwork.AlphaSpaceId)
	c.Assert(err, tc.ErrorIsNil)

	// Act
	err = s.state.SetMachineNetConfig(ctx, nodeUUID, []network.NetInterface{{
		Name:            devName,
		Type:            corenetwork.EthernetDevice,
		VirtualPortType: corenetwork.NonVirtualPort,
		IsAutoStart:     true,
		IsEnabled:       true,
		Addrs: []network.NetAddr{{
			InterfaceName: devName,
			AddressValue:  "192.168.0.50/24",
			AddressType:   corenetwork.IPv4Address,
			ConfigType:    corenetwork.ConfigDHCP,
			Origin:        corenetwork.OriginMachine,
			Scope:         corenetwork.ScopeCloudLocal,
		}},
		DNSSearchDomains: []string{"search.maas.net"},
		DNSAddresses:     []string{"8.8.8.8"},
	}})

	// Assert
	c.Assert(err, tc.ErrorIsNil)

	checkScalarResult(c, db, "SELECT name FROM link_layer_device", "eth0")
	checkScalarResult(c, db, "SELECT address_value FROM ip_address", "192.168.0.50/24")
	checkScalarResult(c, db, "SELECT subnet_uuid FROM ip_address", subnetUUID)
	checkScalarResult(c, db, "SELECT search_domain FROM link_layer_device_dns_domain", "search.maas.net")
	checkScalarResult(c, db, "SELECT dns_address FROM link_layer_device_dns_address", "8.8.8.8")
}

func (s *netConfigSuite) TestSetMachineNetConfigMultipleSubnetMatch(c *tc.C) {
	db := s.DB()

	// Arrange
	nodeUUID := "net-node-uuid"
	devName := "eth0"
	subnetUUID1 := "subnet-uuid-1"
	subnetUUID2 := "subnet-uuid-2"

	ctx := c.Context()

	_, err := db.ExecContext(ctx, "INSERT INTO net_node (uuid) VALUES (?)", nodeUUID)
	c.Assert(err, tc.ErrorIsNil)

	for _, subnetUUID := range []string{subnetUUID1, subnetUUID2} {
		_, err = db.ExecContext(ctx, "INSERT INTO subnet (uuid, cidr, space_uuid) VALUES (?, ?, ?)",
			subnetUUID, "192.168.0.0/24", corenetwork.AlphaSpaceId)
		c.Assert(err, tc.ErrorIsNil)
	}

	// Act
	err = s.state.SetMachineNetConfig(ctx, nodeUUID, []network.NetInterface{{
		Name:            devName,
		Type:            corenetwork.EthernetDevice,
		VirtualPortType: corenetwork.NonVirtualPort,
		IsAutoStart:     true,
		IsEnabled:       true,
		Addrs: []network.NetAddr{{
			InterfaceName: devName,
			AddressValue:  "192.168.0.50/24",
			AddressType:   corenetwork.IPv4Address,
			ConfigType:    corenetwork.ConfigDHCP,
			Origin:        corenetwork.OriginMachine,
			Scope:         corenetwork.ScopeCloudLocal,
		}},
		DNSSearchDomains: []string{"search.maas.net"},
		DNSAddresses:     []string{"8.8.8.8"},
	}})

	// Assert
	c.Assert(err, tc.ErrorIsNil)

	checkScalarResult(c, db, "SELECT name FROM link_layer_device", "eth0")
	checkScalarResult(c, db, "SELECT address_value FROM ip_address", "192.168.0.50/24")
	checkScalarResult(c, db, "SELECT search_domain FROM link_layer_device_dns_domain", "search.maas.net")
	checkScalarResult(c, db, "SELECT dns_address FROM link_layer_device_dns_address", "8.8.8.8")

	// Check that we created a new subnet and linked it to the address.
	row := db.QueryRowContext(ctx, "SELECT uuid, cidr FROM subnet WHERE uuid NOT IN (?, ?)", subnetUUID1, subnetUUID2)
	c.Assert(row.Err(), tc.ErrorIsNil)

	var newSubUUID, cidr string
	err = row.Scan(&newSubUUID, &cidr)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(cidr, tc.Equals, "192.168.0.50/32")

	checkScalarResult(c, db, "SELECT subnet_uuid FROM ip_address", newSubUUID)
}

func (s *netConfigSuite) TestSetMachineNetConfigNoAddresses(c *tc.C) {
	db := s.DB()

	// Arrange
	nodeUUID := "net-node-uuid"
	devName := "eth0"

	ctx := c.Context()

	_, err := db.ExecContext(ctx, "INSERT INTO net_node (uuid) VALUES (?)", nodeUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Act
	err = s.state.SetMachineNetConfig(ctx, nodeUUID, []network.NetInterface{{
		Name:            devName,
		Type:            corenetwork.EthernetDevice,
		VirtualPortType: corenetwork.NonVirtualPort,
		IsAutoStart:     true,
		IsEnabled:       true,
	}})

	// Assert
	c.Assert(err, tc.ErrorIsNil)

	checkScalarResult(c, db, "SELECT name FROM link_layer_device", "eth0")

	row := db.QueryRowContext(ctx, "SELECT count(*) FROM ip_address")
	c.Assert(row.Err(), tc.ErrorIsNil)

	var addrCount int
	err = row.Scan(&addrCount)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(addrCount, tc.Equals, 0)
}

func (s *netConfigSuite) TestSetMachineNetConfigUpdatedNIC(c *tc.C) {
	db := s.DB()

	// Arrange
	nodeUUID := "net-node-uuid"
	devName := "eth0"

	ctx := c.Context()

	_, err := db.ExecContext(ctx, "INSERT INTO net_node (uuid) VALUES (?)", nodeUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Act: insert then update.
	nic := network.NetInterface{
		Name:            devName,
		Type:            corenetwork.EthernetDevice,
		VirtualPortType: corenetwork.NonVirtualPort,
		IsAutoStart:     true,
		IsEnabled:       true,
	}

	err = s.state.SetMachineNetConfig(ctx, nodeUUID, []network.NetInterface{nic})
	c.Assert(err, tc.ErrorIsNil)

	nic.VLANTag = uint64(30)
	err = s.state.SetMachineNetConfig(ctx, nodeUUID, []network.NetInterface{nic})

	// Assert
	c.Assert(err, tc.ErrorIsNil)
	checkScalarResult(c, db, "SELECT vlan_tag FROM link_layer_device", "30")
}

func (s *netConfigSuite) TestSetMachineNetConfigWithParentDevices(c *tc.C) {
	db := s.DB()

	// Arrange
	nodeUUID := "net-node-uuid"
	devName := "eth0"
	brName := "br0"

	ctx := c.Context()

	_, err := db.ExecContext(ctx, "INSERT INTO net_node (uuid) VALUES (?)", nodeUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Act
	err = s.state.SetMachineNetConfig(ctx, nodeUUID, []network.NetInterface{
		{
			Name:             devName,
			Type:             corenetwork.EthernetDevice,
			VirtualPortType:  corenetwork.NonVirtualPort,
			IsAutoStart:      true,
			IsEnabled:        true,
			ParentDeviceName: brName,
		},
		{
			Name:            brName,
			Type:            corenetwork.BridgeDevice,
			VirtualPortType: corenetwork.NonVirtualPort,
			IsAutoStart:     true,
			IsEnabled:       true,
		},
	})

	// Assert
	c.Assert(err, tc.ErrorIsNil)

	parentSQL := `
SELECT dp.name 
FROM   link_layer_device AS dp
	   JOIN link_layer_device_parent AS p ON dp.uuid = p.parent_uuid
	   JOIN link_layer_device AS dc ON p.device_uuid = dc.uuid	
WHERE  dc.name = 'eth0'`

	checkScalarResult(c, db, parentSQL, brName)
}

func (s *netConfigSuite) TestSetMachineNetConfigUpdateConfigType(c *tc.C) {
	db := s.DB()

	// Arrange
	nodeUUID := "net-node-uuid"
	devName := "eth0"
	subnetUUID := "subnet-uuid"

	ctx := c.Context()

	_, err := db.ExecContext(ctx, "INSERT INTO net_node (uuid) VALUES (?)", nodeUUID)
	c.Assert(err, tc.ErrorIsNil)

	_, err = db.ExecContext(ctx, "INSERT INTO subnet (uuid, cidr, space_uuid) VALUES (?, ?, ?)",
		subnetUUID, "192.168.0.0/24", corenetwork.AlphaSpaceId)
	c.Assert(err, tc.ErrorIsNil)

	// Act: set a device and address then set again with a
	// different address config type.
	netConfig := []network.NetInterface{{
		Name:            devName,
		Type:            corenetwork.EthernetDevice,
		VirtualPortType: corenetwork.NonVirtualPort,
		IsAutoStart:     true,
		IsEnabled:       true,
		Addrs: []network.NetAddr{{
			InterfaceName: devName,
			AddressValue:  "192.168.0.50/24",
			AddressType:   corenetwork.IPv4Address,
			ConfigType:    corenetwork.ConfigDHCP,
			Origin:        corenetwork.OriginMachine,
			Scope:         corenetwork.ScopeCloudLocal,
		}},
		DNSSearchDomains: []string{"search.maas.net"},
		DNSAddresses:     []string{"8.8.8.8"},
	}}

	err = s.state.SetMachineNetConfig(ctx, nodeUUID, netConfig)
	c.Assert(err, tc.ErrorIsNil)

	netConfig[0].Addrs[0].ConfigType = corenetwork.ConfigStatic
	err = s.state.SetMachineNetConfig(ctx, nodeUUID, netConfig)

	// Assert
	c.Assert(err, tc.ErrorIsNil)

	checkScalarResult(c, db, "SELECT config_type_id FROM ip_address", "4")
}

func (s *netConfigSuite) TestSetMachineNetConfigUpdateProviderAddressMovesDevices(c *tc.C) {
	db := s.DB()

	// Arrange: set a device with an address, then give it a provider origin.
	nodeUUID := "net-node-uuid"
	devName := "eth0"
	subnetUUID := "subnet-uuid"

	ctx := c.Context()

	_, err := db.ExecContext(ctx, "INSERT INTO net_node (uuid) VALUES (?)", nodeUUID)
	c.Assert(err, tc.ErrorIsNil)

	_, err = db.ExecContext(ctx, "INSERT INTO subnet (uuid, cidr, space_uuid) VALUES (?, ?, ?)",
		subnetUUID, "192.168.0.0/24", corenetwork.AlphaSpaceId)
	c.Assert(err, tc.ErrorIsNil)

	netConfig := []network.NetInterface{
		{
			Name:            devName,
			Type:            corenetwork.EthernetDevice,
			VirtualPortType: corenetwork.NonVirtualPort,
			IsAutoStart:     true,
			IsEnabled:       true,
			Addrs: []network.NetAddr{{
				InterfaceName: devName,
				AddressValue:  "192.168.0.50/24",
				AddressType:   corenetwork.IPv4Address,
				ConfigType:    corenetwork.ConfigDHCP,
				Origin:        corenetwork.OriginMachine,
				Scope:         corenetwork.ScopeCloudLocal,
			}},
			DNSSearchDomains: []string{"search.maas.net"},
			DNSAddresses:     []string{"8.8.8.8"},
		},
	}

	err = s.state.SetMachineNetConfig(ctx, nodeUUID, netConfig)
	c.Assert(err, tc.ErrorIsNil)

	_, err = db.ExecContext(ctx, "UPDATE ip_address SET origin_id = 1")
	c.Assert(err, tc.ErrorIsNil)

	// Act: set the config again, but now with a
	// bridge that the address has moved to.
	brName := "br-eth0"

	netConfig = []network.NetInterface{
		{
			Name:            brName,
			Type:            corenetwork.BridgeDevice,
			VirtualPortType: corenetwork.NonVirtualPort,
			IsAutoStart:     true,
			IsEnabled:       true,
			Addrs: []network.NetAddr{{
				InterfaceName: brName,
				AddressValue:  "192.168.0.50/24",
				AddressType:   corenetwork.IPv4Address,
				ConfigType:    corenetwork.ConfigDHCP,
				Origin:        corenetwork.OriginMachine,
				Scope:         corenetwork.ScopeCloudLocal,
			}},
			DNSSearchDomains: []string{"search.maas.net"},
			DNSAddresses:     []string{"8.8.8.8"},
		},
		{
			Name:             devName,
			Type:             corenetwork.EthernetDevice,
			VirtualPortType:  corenetwork.NonVirtualPort,
			IsAutoStart:      true,
			IsEnabled:        true,
			DNSSearchDomains: []string{"search.maas.net"},
			DNSAddresses:     []string{"8.8.8.8"},
		},
	}

	err = s.state.SetMachineNetConfig(ctx, nodeUUID, netConfig)

	// Assert: the address should be against the new bridge device.
	c.Assert(err, tc.ErrorIsNil)

	q := `
SELECT d.name
FROM   link_layer_device d
       JOIN ip_address a ON d.uuid = a.device_uuid
WHERE  a.address_value = '192.168.0.50/24'`

	checkScalarResult(c, db, q, brName)
}

func (s *netConfigSuite) TestSetMachineNetConfigLinkedSubnetWithDifferentCIDRNotUpdated(c *tc.C) {
	db := s.DB()

	// Arrange
	nodeUUID := "net-node-uuid"
	devName := "eth0"
	subnetUUID := "subnet-uuid"

	ctx := c.Context()

	_, err := db.ExecContext(ctx, "INSERT INTO net_node (uuid) VALUES (?)", nodeUUID)
	c.Assert(err, tc.ErrorIsNil)

	_, err = db.ExecContext(ctx, "INSERT INTO subnet (uuid, cidr, space_uuid) VALUES (?, ?, ?)",
		subnetUUID, "192.168.0.0/24", corenetwork.AlphaSpaceId)
	c.Assert(err, tc.ErrorIsNil)

	// Act: set a device and address, change its linked subnet's CIDR,
	// then attempt to update the address.
	netConfig := []network.NetInterface{{
		Name:            devName,
		Type:            corenetwork.EthernetDevice,
		VirtualPortType: corenetwork.NonVirtualPort,
		IsAutoStart:     true,
		IsEnabled:       true,
		Addrs: []network.NetAddr{{
			InterfaceName: devName,
			AddressValue:  "192.168.0.50/24",
			AddressType:   corenetwork.IPv4Address,
			ConfigType:    corenetwork.ConfigDHCP,
			Origin:        corenetwork.OriginMachine,
			Scope:         corenetwork.ScopeCloudLocal,
		}},
		DNSSearchDomains: []string{"search.maas.net"},
		DNSAddresses:     []string{"8.8.8.8"},
	}}

	err = s.state.SetMachineNetConfig(ctx, nodeUUID, netConfig)
	c.Assert(err, tc.ErrorIsNil)

	_, err = db.ExecContext(ctx, "UPDATE subnet SET cidr = '192.168.5.0/24'")
	c.Assert(err, tc.ErrorIsNil)

	err = s.state.SetMachineNetConfig(ctx, nodeUUID, netConfig)

	// Assert: address subnet is unchanged.
	// This is contrived, but it ensures that an address already linked to a
	// subnet does not add a /32 or /128 CIDR just because network matching
	// does not place the address in the subnet.
	c.Assert(err, tc.ErrorIsNil)

	checkScalarResult(c, db, "SELECT subnet_uuid FROM ip_address", subnetUUID)
}

func (s *netConfigSuite) TestSetMachineNetConfigDeleteAddressAndDevice(c *tc.C) {
	db := s.DB()

	// Arrange: run an update to get an existing device and address.
	nodeUUID := "net-node-uuid"
	devName := "eth0"
	newDevName := "eth1"
	subnetUUID := "subnet-uuid"

	ctx := c.Context()

	_, err := db.ExecContext(ctx, "INSERT INTO net_node (uuid) VALUES (?)", nodeUUID)
	c.Assert(err, tc.ErrorIsNil)

	_, err = db.ExecContext(ctx, "INSERT INTO subnet (uuid, cidr, space_uuid) VALUES (?, ?, ?)",
		subnetUUID, "192.168.0.0/24", corenetwork.AlphaSpaceId)
	c.Assert(err, tc.ErrorIsNil)

	err = s.state.SetMachineNetConfig(ctx, nodeUUID, []network.NetInterface{{
		Name:            devName,
		Type:            corenetwork.EthernetDevice,
		VirtualPortType: corenetwork.NonVirtualPort,
		IsAutoStart:     true,
		IsEnabled:       true,
		Addrs: []network.NetAddr{{
			InterfaceName: devName,
			AddressValue:  "192.168.0.50/24",
			AddressType:   corenetwork.IPv4Address,
			ConfigType:    corenetwork.ConfigDHCP,
			Origin:        corenetwork.OriginMachine,
			Scope:         corenetwork.ScopeCloudLocal,
		}},
		DNSSearchDomains: []string{"search.maas.net"},
		DNSAddresses:     []string{"8.8.8.8"},
	}})
	c.Assert(err, tc.ErrorIsNil)

	// Act: send a different NIC with no addresses.
	err = s.state.SetMachineNetConfig(ctx, nodeUUID, []network.NetInterface{{
		Name:             newDevName,
		Type:             corenetwork.EthernetDevice,
		VirtualPortType:  corenetwork.NonVirtualPort,
		IsAutoStart:      true,
		IsEnabled:        true,
		DNSSearchDomains: []string{"search.maas.net"},
		DNSAddresses:     []string{"8.8.8.8"},
	}})

	// Assert: the original device (and by implication its addresses) is gone.
	c.Assert(err, tc.ErrorIsNil)

	checkScalarResult(c, db, "SELECT name FROM link_layer_device", newDevName)
}

func (s *netConfigSuite) TestSetMachineNetConfigProviderAddressNotDeleted(c *tc.C) {
	db := s.DB()

	// Arrange: run an update to get an existing device and address,
	// then ensure the address has a provider origin.
	nodeUUID := "net-node-uuid"
	devName := "eth0"
	newDevName := "eth1"
	subnetUUID := "subnet-uuid"

	ctx := c.Context()

	_, err := db.ExecContext(ctx, "INSERT INTO net_node (uuid) VALUES (?)", nodeUUID)
	c.Assert(err, tc.ErrorIsNil)

	_, err = db.ExecContext(ctx, "INSERT INTO subnet (uuid, cidr, space_uuid) VALUES (?, ?, ?)",
		subnetUUID, "192.168.0.0/24", corenetwork.AlphaSpaceId)
	c.Assert(err, tc.ErrorIsNil)

	err = s.state.SetMachineNetConfig(ctx, nodeUUID, []network.NetInterface{{
		Name:            devName,
		Type:            corenetwork.EthernetDevice,
		VirtualPortType: corenetwork.NonVirtualPort,
		IsAutoStart:     true,
		IsEnabled:       true,
		Addrs: []network.NetAddr{{
			InterfaceName: devName,
			AddressValue:  "192.168.0.50/24",
			AddressType:   corenetwork.IPv4Address,
			ConfigType:    corenetwork.ConfigDHCP,
			Origin:        corenetwork.OriginMachine,
			Scope:         corenetwork.ScopeCloudLocal,
		}},
		DNSSearchDomains: []string{"search.maas.net"},
		DNSAddresses:     []string{"8.8.8.8"},
	}})
	c.Assert(err, tc.ErrorIsNil)

	_, err = db.ExecContext(ctx, "UPDATE ip_address SET origin_id = 1 WHERE net_node_uuid = ?", nodeUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Act: send a different NIC with no addresses.
	err = s.state.SetMachineNetConfig(ctx, nodeUUID, []network.NetInterface{{
		Name:             newDevName,
		Type:             corenetwork.EthernetDevice,
		VirtualPortType:  corenetwork.NonVirtualPort,
		IsAutoStart:      true,
		IsEnabled:        true,
		DNSSearchDomains: []string{"search.maas.net"},
		DNSAddresses:     []string{"8.8.8.8"},
	}})

	// Assert: the original device did not have its address deleted,
	// and the device remains in the database.
	c.Assert(err, tc.ErrorIsNil)

	checkScalarResult(c, db, "SELECT name FROM link_layer_device WHERE name = 'eth0'", devName)
}

func (s *netConfigSuite) TestSetMachineNetConfigParentNotDeleted(c *tc.C) {
	db := s.DB()

	// Arrange: run an update to get an existing parent and child device.
	nodeUUID := "net-node-uuid"
	devName := "br-eth"
	parentDevName := "eth0"
	subnetUUID := "subnet-uuid"

	ctx := c.Context()

	_, err := db.ExecContext(ctx, "INSERT INTO net_node (uuid) VALUES (?)", nodeUUID)
	c.Assert(err, tc.ErrorIsNil)

	_, err = db.ExecContext(ctx, "INSERT INTO subnet (uuid, cidr, space_uuid) VALUES (?, ?, ?)",
		subnetUUID, "192.168.0.0/24", corenetwork.AlphaSpaceId)
	c.Assert(err, tc.ErrorIsNil)

	err = s.state.SetMachineNetConfig(ctx, nodeUUID, []network.NetInterface{
		{
			Name:             parentDevName,
			Type:             corenetwork.EthernetDevice,
			VirtualPortType:  corenetwork.NonVirtualPort,
			IsAutoStart:      true,
			IsEnabled:        true,
			DNSSearchDomains: []string{"search.maas.net"},
			DNSAddresses:     []string{"8.8.8.8"},
		},
		{
			Name:             devName,
			Type:             corenetwork.BridgeDevice,
			VirtualPortType:  corenetwork.NonVirtualPort,
			IsAutoStart:      true,
			IsEnabled:        true,
			DNSSearchDomains: []string{"search.maas.net"},
			DNSAddresses:     []string{"8.8.8.8"},
			ParentDeviceName: parentDevName,
		},
	})
	c.Assert(err, tc.ErrorIsNil)

	_, err = db.ExecContext(ctx, "UPDATE ip_address SET origin_id = 1 WHERE net_node_uuid = ?", nodeUUID)
	c.Assert(err, tc.ErrorIsNil)

	// Act: send only the child device in an update.
	err = s.state.SetMachineNetConfig(ctx, nodeUUID, []network.NetInterface{{
		Name:             devName,
		Type:             corenetwork.BridgeDevice,
		VirtualPortType:  corenetwork.NonVirtualPort,
		IsAutoStart:      true,
		IsEnabled:        true,
		DNSSearchDomains: []string{"search.maas.net"},
		DNSAddresses:     []string{"8.8.8.8"},
		ParentDeviceName: parentDevName,
	}})

	// Assert: the parent device, though not observed in the update,
	// should not be deleted.
	c.Assert(err, tc.ErrorIsNil)

	checkScalarResult(c, db, "SELECT name FROM link_layer_device WHERE name = 'eth0'", parentDevName)
}

func checkScalarResult(c *tc.C, db *sql.DB, query string, expected string) {
	rows, err := db.QueryContext(c.Context(), query)
	c.Assert(err, tc.ErrorIsNil)
	defer func() { _ = rows.Close() }()

	var (
		actual   string
		rowCount int
	)

	for rows.Next() {
		err = rows.Scan(&actual)
		c.Assert(err, tc.ErrorIsNil)
		rowCount++
	}

	c.Assert(rowCount, tc.Equals, 1)
	c.Check(actual, tc.Equals, expected)
}
