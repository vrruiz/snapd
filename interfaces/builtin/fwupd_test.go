// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2016 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package builtin_test

import (
	. "gopkg.in/check.v1"

	"github.com/snapcore/snapd/interfaces"
	"github.com/snapcore/snapd/interfaces/apparmor"
	"github.com/snapcore/snapd/interfaces/builtin"
	"github.com/snapcore/snapd/interfaces/seccomp"
	"github.com/snapcore/snapd/snap"
	"github.com/snapcore/snapd/snap/snaptest"
	"github.com/snapcore/snapd/testutil"
)

type FwupdInterfaceSuite struct {
	iface interfaces.Interface
	slot  *interfaces.Slot
	plug  *interfaces.Plug
}

const mockPlugSnapInfoYaml = `name: uefi-fw-tools
version: 1.0
apps:
 app:
  command: foo
  plugs: [fwupd]
`

const mockSlotSnapInfoYaml = `name: uefi-fw-tools
version: 1.0
apps:
 app2:
  command: foo
  slots: [fwupd]
`

var _ = Suite(&FwupdInterfaceSuite{})

func (s *FwupdInterfaceSuite) SetUpTest(c *C) {
	s.iface = &builtin.FwupdInterface{}
	slotSnap := snaptest.MockInfo(c, mockSlotSnapInfoYaml, nil)
	plugSnap := snaptest.MockInfo(c, mockPlugSnapInfoYaml, nil)
	s.slot = &interfaces.Slot{SlotInfo: slotSnap.Slots["fwupd"]}
	s.plug = &interfaces.Plug{PlugInfo: plugSnap.Plugs["fwupd"]}
}

func (s *FwupdInterfaceSuite) TestName(c *C) {
	c.Assert(s.iface.Name(), Equals, "fwupd")
}

// The label glob when all apps are bound to the fwupd slot
func (s *FwupdInterfaceSuite) TestConnectedPlugSnippetUsesSlotLabelAll(c *C) {
	app1 := &snap.AppInfo{Name: "app1"}
	app2 := &snap.AppInfo{Name: "app2"}
	slot := &interfaces.Slot{
		SlotInfo: &snap.SlotInfo{
			Snap: &snap.Info{
				SuggestedName: "uefi-fw-tools",
				Apps:          map[string]*snap.AppInfo{"app1": app1, "app2": app2},
			},
			Name:      "fwupd",
			Interface: "fwupd",
			Apps:      map[string]*snap.AppInfo{"app1": app1, "app2": app2},
		},
	}

	// connected plugs have a non-nil security snippet for apparmor
	apparmorSpec := &apparmor.Specification{}
	err := apparmorSpec.AddConnectedPlug(s.iface, s.plug, slot)
	c.Assert(err, IsNil)
	c.Assert(apparmorSpec.SecurityTags(), DeepEquals, []string{"snap.uefi-fw-tools.app"})
	c.Assert(apparmorSpec.SnippetForTag("snap.uefi-fw-tools.app"), testutil.Contains, `peer=(label="snap.uefi-fw-tools.*"),`)
}

// The label uses alternation when some, but not all, apps is bound to the fwupd slot
func (s *FwupdInterfaceSuite) TestConnectedPlugSnippetUsesSlotLabelSome(c *C) {
	app1 := &snap.AppInfo{Name: "app1"}
	app2 := &snap.AppInfo{Name: "app2"}
	app3 := &snap.AppInfo{Name: "app3"}
	slot := &interfaces.Slot{
		SlotInfo: &snap.SlotInfo{
			Snap: &snap.Info{
				SuggestedName: "uefi-fw-tools",
				Apps:          map[string]*snap.AppInfo{"app1": app1, "app2": app2, "app3": app3},
			},
			Name:      "fwupd",
			Interface: "fwupd",
			Apps:      map[string]*snap.AppInfo{"app1": app1, "app2": app2},
		},
	}

	apparmorSpec := &apparmor.Specification{}
	err := apparmorSpec.AddConnectedPlug(s.iface, s.plug, slot)
	c.Assert(err, IsNil)
	c.Assert(apparmorSpec.SecurityTags(), DeepEquals, []string{"snap.uefi-fw-tools.app"})
	c.Assert(apparmorSpec.SnippetForTag("snap.uefi-fw-tools.app"), testutil.Contains, `peer=(label="snap.uefi-fw-tools.{app1,app2}"),`)
}

// The label uses short form when exactly one app is bound to the fwupd slot
func (s *FwupdInterfaceSuite) TestConnectedPlugSnippetUsesSlotLabelOne(c *C) {
	apparmorSpec := &apparmor.Specification{}
	err := apparmorSpec.AddConnectedPlug(s.iface, s.plug, s.slot)
	c.Assert(err, IsNil)
	c.Assert(apparmorSpec.SecurityTags(), DeepEquals, []string{"snap.uefi-fw-tools.app"})
	c.Assert(apparmorSpec.SnippetForTag("snap.uefi-fw-tools.app"), testutil.Contains, `peer=(label="snap.uefi-fw-tools.app2"),`)
}

func (s *FwupdInterfaceSuite) TestUsedSecuritySystems(c *C) {
	// connected plugs have a non-nil security snippet for apparmor
	apparmorSpec := &apparmor.Specification{}
	err := apparmorSpec.AddConnectedPlug(s.iface, s.plug, s.slot)
	c.Assert(err, IsNil)
	err = apparmorSpec.AddConnectedSlot(s.iface, s.plug, s.slot)
	c.Assert(err, IsNil)
	err = apparmorSpec.AddPermanentSlot(s.iface, s.slot)
	c.Assert(err, IsNil)
	c.Assert(apparmorSpec.SecurityTags(), DeepEquals, []string{"snap.uefi-fw-tools.app", "snap.uefi-fw-tools.app2"})

	snippet, err := s.iface.PermanentSlotSnippet(s.slot, interfaces.SecurityDBus)
	c.Assert(err, IsNil)
	c.Assert(snippet, Not(IsNil))
}

func (s *FwupdInterfaceSuite) TestPermanentSlotSnippetSecComp(c *C) {
	seccompSpec := &seccomp.Specification{}
	err := seccompSpec.AddPermanentSlot(s.iface, s.slot)
	c.Assert(err, IsNil)
	c.Assert(seccompSpec.SecurityTags(), DeepEquals, []string{"snap.uefi-fw-tools.app2"})
	c.Check(seccompSpec.SnippetForTag("snap.uefi-fw-tools.app2"), testutil.Contains, "bind\n")
}

func (s *FwupdInterfaceSuite) TestConnectedPlugSnippetSecComp(c *C) {
	seccompSpec := &seccomp.Specification{}
	err := seccompSpec.AddConnectedPlug(s.iface, s.plug, s.slot)
	c.Assert(err, IsNil)
	c.Assert(seccompSpec.SecurityTags(), DeepEquals, []string{"snap.uefi-fw-tools.app"})
	c.Check(seccompSpec.SnippetForTag("snap.uefi-fw-tools.app"), testutil.Contains, "bind\n")
}
