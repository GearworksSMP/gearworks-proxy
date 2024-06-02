package main

import (
	"context"
	"fmt"
	"github.com/robinbraemer/event"
	"go.minekube.com/brigodier"
	"go.minekube.com/common/minecraft/color"
	"go.minekube.com/common/minecraft/component"
	"go.minekube.com/common/minecraft/component/codec/legacy"
	"go.minekube.com/gate/cmd/gate"
	"go.minekube.com/gate/pkg/command"
	"go.minekube.com/gate/pkg/edition/java/bossbar"
	"go.minekube.com/gate/pkg/edition/java/proxy"
	"go.minekube.com/gate/pkg/edition/java/title"
	"time"
)

func main() {
	// Add our "plug-in" to be initialized on Gate start.
	proxy.Plugins = append(proxy.Plugins, proxy.Plugin{
		Name: "SimpleProxy",
		Init: func(ctx context.Context, proxy *proxy.Proxy) error {
			return newSimpleProxy(proxy).init() // see code examples
		},
	})

	// Execute Gate entrypoint and block until shutdown.
	// We could also run gate.Start if we don't need Gate's command-line.
	gate.Execute()
}

// SimpleProxy is a simple proxy to showcase some features of Gate.
//
// In this example:
//   - Add a `/broadcast` command
//   - Send a message when player switches the server
//   - Show boss bars to players
type SimpleProxy struct {
	*proxy.Proxy
}

var legacyCodec = &legacy.Legacy{Char: legacy.AmpersandChar}

func newSimpleProxy(proxy *proxy.Proxy) *SimpleProxy {
	return &SimpleProxy{
		Proxy: proxy,
	}
}

// initialize our sample proxy
func (p *SimpleProxy) init() error {
	p.registerCommands()
	p.registerSubscribers()
	return nil
}

// Register a proxy-wide commands (can be run while being on any server)
func (p *SimpleProxy) registerCommands() {
	// Registers the "/broadcast" command
	p.Command().Register(brigodier.Literal("broadcast").Then(
		// Adds message argument as in "/broadcast <message>"
		brigodier.Argument("message", brigodier.StringPhrase).
			// Adds completion suggestions as in "/broadcast [suggestions]"
			Suggests(command.SuggestFunc(func(
				c *command.Context,
				b *brigodier.SuggestionsBuilder,
			) *brigodier.Suggestions {
				player, ok := c.Source.(proxy.Player)
				if ok {
					b.Suggest("&oI am &6&l" + player.Username())
				}
				b.Suggest("Hello world!")

				// We can also order suggestion based on the current input and sort them.
				// The slice provides available suggestion candidates.
				//return suggest.Similar(b, []string{"FirstTry", "SecondTry"}).Build()

				return b.Build()
			})).
			// Executed when running "/broadcast <message>"
			Executes(command.Command(func(c *command.Context) error {
				// Colorize/format message
				message, err := legacyCodec.Unmarshal([]byte(c.String("message")))
				if err != nil {
					return c.Source.SendMessage(&component.Text{
						Content: fmt.Sprintf("Error formatting message: %v", err)})
				}

				// Send to all players on this proxy
				for _, player := range p.Players() {
					// Send message in new goroutine to not block
					// this loop if any player has a slow connection.
					go func(p proxy.Player) { _ = p.SendMessage(message) }(player)
				}
				return nil
			})),
	))
	p.Command().Register(brigodier.Literal("ping").
		Executes(command.Command(func(c *command.Context) error {
			player, ok := c.Source.(proxy.Player)
			if !ok {
				return c.Source.SendMessage(&component.Text{Content: "Pong!"})
			}
			return player.SendMessage(&component.Text{
				Content: fmt.Sprintf("Pong! Your ping is %s", player.Ping()),
				S:       component.Style{Color: color.Green},
			})
		})),
	)
	p.Command().Register(titleCommand())
}

func titleCommand() brigodier.LiteralNodeBuilder {
	showTitle := command.Command(func(c *command.Context) error {
		player, ok := c.Source.(proxy.Player)
		if !ok {
			return c.Source.SendMessage(&component.Text{Content: "You must be a player to run this command."})
		}

		ti, err := legacyCodec.Unmarshal([]byte(c.String("title")))
		if err != nil {
			return player.SendMessage(&component.Text{
				Content: fmt.Sprintf("Error parsing title: %v", err),
			})
		}

		// empty if arg not provided
		subtitle, err := legacyCodec.Unmarshal([]byte(c.String("subtitle")))
		if err != nil {
			return player.SendMessage(&component.Text{
				Content: fmt.Sprintf("Error parsing title: %v", err),
			})
		}

		return title.ShowTitle(player, &title.Options{
			Title:    ti,
			Subtitle: subtitle,
		})
	})

	return brigodier.Literal("title").
		Then(brigodier.Argument("title", brigodier.String).Executes(showTitle).
			Then(brigodier.Argument("subtitle", brigodier.StringPhrase).Executes(showTitle)))
}

// Register event subscribers
func (p *SimpleProxy) registerSubscribers() {
	// Send message on server switch.
	event.Subscribe(p.Event(), 0, p.onServerSwitch)

	// Change the MOTD response.
	event.Subscribe(p.Event(), 0, pingHandler())

	// Show a boss bar to all players on this proxy.
	event.Subscribe(p.Event(), 0, p.bossBarDisplay())
}

func (p *SimpleProxy) onServerSwitch(e *proxy.ServerPostConnectEvent) {
	newServer := e.Player().CurrentServer()
	if newServer == nil {
		return
	}

	_ = e.Player().SendMessage(&component.Text{
		S: component.Style{Color: color.Aqua},
		Extra: []component.Component{
			&component.Text{
				Content: "\nWelcome to the Gate Sample proxy!\n\n",
				S:       component.Style{Color: color.Green, Bold: component.True},
			},
			&component.Text{Content: "You connected to "},
			&component.Text{Content: newServer.Server().ServerInfo().Name(), S: component.Style{Color: color.Yellow}},
			&component.Text{Content: "."},
			&component.Text{
				S: component.Style{
					ClickEvent: component.SuggestCommand("/broadcast Gate is awesome!"),
					HoverEvent: component.ShowText(&component.Text{Content: "/broadcast Gate is awesome!"}),
				},
				Content: "\n\nClick me to run ",
				Extra: []component.Component{&component.Text{
					Content: "/broadcast Gate is awesome!",
					S:       component.Style{Color: color.White, Bold: component.True, Italic: component.True},
				}},
			},
			&component.Text{
				Content: "\n\nClick me to run sample /title command!",
				S: component.Style{
					HoverEvent: component.ShowText(&component.Text{Content: "/title <title> [subtitle]"}),
					ClickEvent: component.SuggestCommand(`/title "&eGate greets" &2&o` + e.Player().Username()),
				},
			},
			&component.Text{Content: "\n\nMore sample commands you can try: "},
			&component.Text{
				Content: "/ping",
				S:       component.Style{Color: color.Yellow},
			},
		},
	})
}

func pingHandler() func(p *proxy.PingEvent) {
	motd := &component.Text{Content: "Simple Proxy!\nJoin and test me."}
	return func(e *proxy.PingEvent) {
		p := e.Ping()
		p.Description = motd
		p.Players.Max = p.Players.Online + 1
	}
}

func (p *SimpleProxy) bossBarDisplay() func(*proxy.PostLoginEvent) {
	// Create shared boss bar for all players
	sharedBar := bossbar.New(
		&component.Text{Content: "Welcome to Gate Sample proxy!", S: component.Style{
			Color: color.Aqua,
			Bold:  component.True,
		}},
		1,
		bossbar.BlueColor,
		bossbar.ProgressOverlay,
	)

	updateBossBar := func(bar bossbar.BossBar, player proxy.Player) {
		now := time.Now()
		text := &component.Text{Extra: []component.Component{
			&component.Text{
				Content: fmt.Sprintf("Hello %s! ", player.Username()),
				S:       component.Style{Color: color.Yellow},
			},
			&component.Text{
				Content: fmt.Sprintf("It's %s", now.Format("15:04:05 PM")),
				S:       component.Style{Color: color.Gold},
			},
		}}
		bar.SetName(text)
		bar.SetPercent(float32(now.Second()) / 60)
	}

	return func(e *proxy.PostLoginEvent) {
		player := e.Player()

		// Add player to shared boss bar
		_ = sharedBar.AddViewer(player)

		// Create own boss bar for player
		playerBar := bossbar.New(
			&component.Text{},
			bossbar.MinProgress,
			bossbar.RedColor,
			bossbar.ProgressOverlay,
		)
		// Show it to player
		_ = playerBar.AddViewer(player)

		// Update boss bars every second until player disconnects.
		// Run in new goroutine to unblock login event handler!
		go tick(player.Context(), time.Second, func() {
			updateBossBar(playerBar, player)
		})
	}
}

// tick runs a function every interval until the context is cancelled.
func tick(ctx context.Context, interval time.Duration, fn func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			fn()
		case <-ctx.Done():
			return
		}
	}
}
