// Code generated by mockery v2.12.3. DO NOT EDIT.

package mempool

import (
	net "net"

	mock "github.com/stretchr/testify/mock"
)

// DNSCache is an autogenerated mock type for the DNSCache type
type DNSCache struct {
	mock.Mock
}

// GetDomainIp provides a mock function with given fields: _a0
func (_m *DNSCache) GetDomainIp(_a0 string) ([]net.IPAddr, int64, bool) {
	ret := _m.Called(_a0)

	var r0 []net.IPAddr
	if rf, ok := ret.Get(0).(func(string) []net.IPAddr); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]net.IPAddr)
		}
	}

	var r1 int64
	if rf, ok := ret.Get(1).(func(string) int64); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Get(1).(int64)
	}

	var r2 bool
	if rf, ok := ret.Get(2).(func(string) bool); ok {
		r2 = rf(_a0)
	} else {
		r2 = ret.Get(2).(bool)
	}

	return r0, r1, r2
}

// GetTxtRecord provides a mock function with given fields: _a0
func (_m *DNSCache) GetTxtRecord(_a0 string) ([]string, int64, bool) {
	ret := _m.Called(_a0)

	var r0 []string
	if rf, ok := ret.Get(0).(func(string) []string); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	var r1 int64
	if rf, ok := ret.Get(1).(func(string) int64); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Get(1).(int64)
	}

	var r2 bool
	if rf, ok := ret.Get(2).(func(string) bool); ok {
		r2 = rf(_a0)
	} else {
		r2 = ret.Get(2).(bool)
	}

	return r0, r1, r2
}

// PutDomainIp provides a mock function with given fields: _a0, _a1, _a2
func (_m *DNSCache) PutDomainIp(_a0 string, _a1 []net.IPAddr, _a2 int64) bool {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 bool
	if rf, ok := ret.Get(0).(func(string, []net.IPAddr, int64) bool); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// PutTxtRecord provides a mock function with given fields: _a0, _a1, _a2
func (_m *DNSCache) PutTxtRecord(_a0 string, _a1 []string, _a2 int64) bool {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 bool
	if rf, ok := ret.Get(0).(func(string, []string, int64) bool); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// RemoveIp provides a mock function with given fields: _a0
func (_m *DNSCache) RemoveIp(_a0 string) bool {
	ret := _m.Called(_a0)

	var r0 bool
	if rf, ok := ret.Get(0).(func(string) bool); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// RemoveTxt provides a mock function with given fields: _a0
func (_m *DNSCache) RemoveTxt(_a0 string) bool {
	ret := _m.Called(_a0)

	var r0 bool
	if rf, ok := ret.Get(0).(func(string) bool); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Size provides a mock function with given fields:
func (_m *DNSCache) Size() (uint, uint) {
	ret := _m.Called()

	var r0 uint
	if rf, ok := ret.Get(0).(func() uint); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint)
	}

	var r1 uint
	if rf, ok := ret.Get(1).(func() uint); ok {
		r1 = rf()
	} else {
		r1 = ret.Get(1).(uint)
	}

	return r0, r1
}

type NewDNSCacheT interface {
	mock.TestingT
	Cleanup(func())
}

// NewDNSCache creates a new instance of DNSCache. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewDNSCache(t NewDNSCacheT) *DNSCache {
	mock := &DNSCache{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
