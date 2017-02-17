// Automatically generated by MockGen. DO NOT EDIT!
// Source: github.com/mmatczuk/go-http-tunnel/log (interfaces: Logger)

package mock

import (
	gomock "github.com/golang/mock/gomock"
)

// Mock of Logger interface
type MockLogger struct {
	ctrl     *gomock.Controller
	recorder *_MockLoggerRecorder
}

// Recorder for MockLogger (not exported)
type _MockLoggerRecorder struct {
	mock *MockLogger
}

func NewMockLogger(ctrl *gomock.Controller) *MockLogger {
	mock := &MockLogger{ctrl: ctrl}
	mock.recorder = &_MockLoggerRecorder{mock}
	return mock
}

func (_m *MockLogger) EXPECT() *_MockLoggerRecorder {
	return _m.recorder
}

func (_m *MockLogger) Log(_param0 ...interface{}) error {
	_s := []interface{}{}
	for _, _x := range _param0 {
		_s = append(_s, _x)
	}
	ret := _m.ctrl.Call(_m, "Log", _s...)
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockLoggerRecorder) Log(arg0 ...interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Log", arg0...)
}