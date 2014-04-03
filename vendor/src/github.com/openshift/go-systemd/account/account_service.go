/*
Copyright 2014 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package account

import (
	"errors"
	"fmt"
	"github.com/godbus/dbus"
)

var (
	ErrNoSuchUser      = errors.New("No such user")
	ErrInvalidProperty = errors.New("Invalid property")
)

type AccountService struct {
	dconn         *dbus.Conn
	service       *dbus.Object
	propMethodMap map[string]string
}

func NewAccountService() (*AccountService, error) {
	a := &AccountService{}

	var err error
	a.dconn, err = dbus.SystemBusPrivate()
	if err != nil {
		return nil, err
	}

	err = a.dconn.Auth(nil)
	if err != nil {
		a.dconn.Close()
		return nil, err
	}

	err = a.dconn.Hello()
	if err != nil {
		a.dconn.Close()
		return nil, err
	}

	a.service = a.dconn.Object("org.freedesktop.Accounts", "/org/freedesktop/Accounts")
	a.propMethodMap = map[string]string{
		"RealName":      "SetRealName",
		"Email":         "SetEmail",
		"HomeDirectory": "SetHomeDirectory",
		"Shell":         "SetShell",
		"Locked":        "SetLocked",
		"AccountType":   "SetAccountType",
	}

	return a, nil
}

func (a *AccountService) Close() {
	a.dconn.Close()
}

func (a *AccountService) GetUserByName(name string) (*dbus.Object, error) {
	var user_path dbus.ObjectPath

	a.service.Call("org.freedesktop.Accounts.FindUserByName", 0, name).Store(&user_path)
	if user_path == "" {
		return nil, ErrNoSuchUser
	}

	return a.dconn.Object("org.freedesktop.Accounts", user_path), nil
}

func (a *AccountService) GetUserById(id int) (*dbus.Object, error) {
	var user_path dbus.ObjectPath

	a.service.Call("org.freedesktop.Accounts.FindUserById", 0, id).Store(&user_path)
	if user_path == "" {
		return nil, ErrNoSuchUser
	}

	return a.dconn.Object("org.freedesktop.Accounts", user_path), nil
}

func (a *AccountService) GetUserProperty(user *dbus.Object, propName string) (interface{}, error) {
	methodName := a.propMethodMap[propName]
	if methodName == "" {
		return nil, ErrInvalidProperty
	}

	propValue, err := user.GetProperty(fmt.Sprintf("org.freedesktop.Accounts.User.%v", propName))
	if err != nil {
		return nil, err
	}
	return propValue, nil
}

func (a *AccountService) SetUserProperty(user *dbus.Object, propName string, propValue interface{}) error {
	methodName := a.propMethodMap[propName]
	if methodName == "" {
		return ErrInvalidProperty
	}

	if call := user.Call(fmt.Sprintf("org.freedesktop.Accounts.User.%v", methodName), 0, propValue); call.Err != nil {
		return call.Err
	}
	return nil
}

func (a *AccountService) SetRealName(user *dbus.Object, propValue string) error {
	return a.SetUserProperty(user, "RealName", propValue)
}

func (a *AccountService) SetEmail(user *dbus.Object, propValue string) error {
	return a.SetUserProperty(user, "Email", propValue)
}

func (a *AccountService) SetHomeDirectory(user *dbus.Object, propValue string) error {
	return a.SetUserProperty(user, "HomeDirectory", propValue)
}

func (a *AccountService) SetShell(user *dbus.Object, propValue string) error {
	return a.SetUserProperty(user, "Shell", propValue)
}

func (a *AccountService) SetLocked(user *dbus.Object, propValue bool) error {
	return a.SetUserProperty(user, "Locked", propValue)
}

func (a *AccountService) SetAccountType(user *dbus.Object, propValue int32) error {
	return a.SetUserProperty(user, "AccountType", propValue)
}

func (a *AccountService) CreateUser(name string, fullName string, homeDir string, shell string) (*dbus.Object, error) {
	var err error
	var user *dbus.Object
	var curHomeDir interface{}

	if call := a.service.Call("org.freedesktop.Accounts.CreateUser", 0, name, fullName, int32(0)); call.Err != nil {
		return nil, call.Err
	}

	if user, err = a.GetUserByName(name); err != nil {
		return nil, err
	}

	if curHomeDir, err = a.GetUserProperty(user, "HomeDirectory"); err != nil {
		return nil, err
	}

	if curHomeDir.(dbus.Variant).String() != homeDir {
		if err := a.SetUserProperty(user, "HomeDirectory", homeDir); err != nil {
			return nil, err
		}
	}

	if err := a.SetUserProperty(user, "Shell", shell); err != nil {
		return nil, err
	}

	return user, nil
}
