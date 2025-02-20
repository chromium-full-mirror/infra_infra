// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package botman

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/infra/cmd/drone-agent/internal/bot"
)

func TestBotman(t *testing.T) {
	t.Parallel()
	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		b := bot.NewFakeBot()
		started := make(chan string, 1)
		released := make(chan string, 1)
		h := stubHook{
			start: func(id string) (bot.Bot, error) {
				started <- id
				return b, nil
			},
			release: func(id string) { released <- id },
		}
		c := NewBotman(h)

		const d = "some-dut"
		c.AddBot(d)
		select {
		case got := <-started:
			if got != d {
				t.Errorf("Got started bot %v; want %v", got, d)
			}
		case <-time.After(time.Second):
			t.Fatalf("bot not started after adding ID")
		}

		c.DrainBot(d)
		c.Wait()
		select {
		case got := <-released:
			if got != d {
				t.Errorf("Got released bot %v; want %v", got, d)
			}
		default:
			t.Fatalf("bot not released after draining ID")
		}
	})
	t.Run("active bots", func(t *testing.T) {
		t.Parallel()
		released := make(chan string, 1)
		h := stubHook{
			release: func(id string) { released <- id },
		}
		c := NewBotman(h)
		t.Run("empty before adding", func(t *testing.T) {
			if got := c.ActiveBots(); len(got) != 0 {
				t.Errorf("ActiveBots() = %v; want empty", got)
			}
		})
		const d = "some-dut"
		c.AddBot(d)
		t.Run("added bot is present", func(t *testing.T) {
			want := []string{d}
			got := c.ActiveBots()
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("ActiveBots() mismatch (-want +got):\n%s", diff)
			}
		})
		t.Run("empty after draining", func(t *testing.T) {
			c.DrainBot(d)
			c.Wait()
			if got := c.ActiveBots(); len(got) != 0 {
				t.Errorf("ActiveBots() = %v; want empty", got)
			}
		})
	})
	t.Run("draining missing ID still releases", func(t *testing.T) {
		t.Parallel()
		released := make(chan string, 1)
		h := stubHook{
			release: func(id string) { released <- id },
		}
		c := NewBotman(h)

		const d = "some-dut"
		t.Run("drain", func(t *testing.T) {
			c.DrainBot(d)
			c.Wait()
			select {
			case got := <-released:
				if got != d {
					t.Errorf("Got released bot %v; want %v", got, d)
				}
			default:
				t.Fatalf("bot not released after draining")
			}

		})
		t.Run("terminate", func(t *testing.T) {
			c.TerminateBot(d)
			c.Wait()
			select {
			case got := <-released:
				if got != d {
					t.Errorf("Got released bot %v; want %v", got, d)
				}
			default:
				t.Fatalf("bot not released after draining")
			}

		})
	})
	t.Run("restart bot if crash", func(t *testing.T) {
		t.Parallel()
		started := make(chan *bot.FakeBot, 1)
		h := stubHook{
			start: func(id string) (bot.Bot, error) {
				b := bot.NewFakeBot()
				started <- b
				return b, nil
			},
		}
		c := NewBotman(h)
		defer c.Wait()

		const d = "some-dut"
		c.AddBot(d)
		defer c.TerminateBot(d)
		b := <-started
		b.Stop()
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatalf("bot not restarted after stopping")
		}
	})
	t.Run("can drain ID even if starting errors", func(t *testing.T) {
		t.Parallel()
		h := stubHook{
			start: func(id string) (bot.Bot, error) {
				return nil, errors.New("some error")
			},
		}
		c := NewBotman(h)
		const d = "some-dut"
		c.AddBot(d)
		c.DrainBot(d)
		assertDontHang(t, c.Wait, "Wait hanged")
	})
	t.Run("can terminate ID even if starting errors", func(t *testing.T) {
		t.Parallel()
		h := stubHook{
			start: func(id string) (bot.Bot, error) {
				return nil, errors.New("some error")
			},
		}
		c := NewBotman(h)
		const d = "some-dut"
		c.AddBot(d)
		c.TerminateBot(d)
		assertDontHang(t, c.Wait, "Wait hanged")
	})
	t.Run("drain crashlooping bot still releases", func(t *testing.T) {
		t.Parallel()
		released := make(chan string, 1)
		h := stubHook{
			start: func(id string) (bot.Bot, error) {
				return nil, errors.New("some error")
			},
			release: func(id string) { released <- id },
		}
		c := NewBotman(h)
		const d = "some-dut"
		c.AddBot(d)
		c.DrainBot(d)
		c.Wait()
		select {
		case got := <-released:
			if got != d {
				t.Errorf("Got released bot %v; want %v", got, d)
			}
		case <-time.After(time.Second):
			t.Errorf("Did not release ID")
		}
	})
	t.Run("terminate crashlooping bot still releases", func(t *testing.T) {
		t.Parallel()
		released := make(chan string, 1)
		h := stubHook{
			start: func(id string) (bot.Bot, error) {
				return nil, errors.New("some error")
			},
			release: func(id string) { released <- id },
		}
		c := NewBotman(h)
		const d = "some-dut"
		c.AddBot(d)
		c.TerminateBot(d)
		c.Wait()
		select {
		case got := <-released:
			if got != d {
				t.Errorf("Got released bot %v; want %v", got, d)
			}
		case <-time.After(time.Second):
			t.Errorf("Did not release ID")
		}
	})
	t.Run("stopped IDs are removed", func(t *testing.T) {
		t.Parallel()
		c := NewBotman(stubHook{})
		c.AddBot("ionasal")
		c.DrainBot("ionasal")
		c.Wait()
		got := c.bots
		if len(got) > 0 {
			t.Errorf("Got running IDs %v; want none", got)
		}
	})
	t.Run("drain all does not hang", func(t *testing.T) {
		t.Parallel()
		c := NewBotman(stubHook{})
		c.AddBot("ionasal")
		c.AddBot("nero")
		assertDontHang(t, c.DrainAll, "DrainAll hanged")
		c.Wait()
	})
	t.Run("terminate all does not hang", func(t *testing.T) {
		t.Parallel()
		c := NewBotman(stubHook{})
		c.AddBot("ionasal")
		c.AddBot("nero")
		assertDontHang(t, c.TerminateAll, "TerminateAll hanged")
		c.Wait()
	})
	t.Run("block bots stops add new bot", func(t *testing.T) {
		t.Parallel()
		b := bot.NewFakeBot()
		var m sync.Mutex
		var started int
		h := stubHook{
			start: func(id string) (bot.Bot, error) {
				m.Lock()
				started++
				m.Unlock()
				return b, nil
			},
		}
		c := NewBotman(h)

		c.BlockBots()
		const d = "some-dut"
		c.AddBot(d)
		m.Lock()
		got := started
		m.Unlock()
		if got != 0 {
			t.Errorf("Got %v bots started; want 0", got)
		}
	})
}

func assertDontHang(t *testing.T, f func(), msg string) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		f()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal(msg)
	}
}

// stubHook is an implementation of WorldHook for tests.
type stubHook struct {
	start   func(string) (bot.Bot, error)
	release func(string)
}

func (h stubHook) StartBot(id string) (bot.Bot, error) {
	if f := h.start; f != nil {
		return f(id)
	}
	return bot.NewFakeBot(), nil
}

func (h stubHook) ReleaseResources(id string) {
	if f := h.release; f != nil {
		f(id)
	}
}
