package ui

import "github.com/xmhrnx/giode"

// tappable makes content clickable. Its press state is kept under
// (scope, key), so it can be built inside the view every frame; the
// content and the handler are refreshed each time.
func tappable(scope, key any, onClick func(), content func(s giode.PressState) giode.Element) giode.Element {
	p := giode.Keyed(scope, key, func() *giode.PressableElement {
		return giode.Pressable(content)
	})
	p.Content(content).OnClick(func(giode.ClickEvent) { onClick() })
	return p
}

// growTappable is tappable taking a share of its row's spare width.
func growTappable(scope, key any, onClick func(), content func(s giode.PressState) giode.Element) giode.Element {
	p := giode.Keyed(scope, key, func() *giode.PressableElement {
		return giode.Pressable(content, giode.Styles{FlexGrow: 1})
	})
	p.Content(content).OnClick(func(giode.ClickEvent) { onClick() })
	return p
}

// longTappable is tappable with a long-press (touch) and secondary
// click (mouse) action as well.
func longTappable(scope, key any, onClick, onLong func(), content func(s giode.PressState) giode.Element) giode.Element {
	return longTappableStyled(scope, key, giode.Styles{}, onClick, onLong, content)
}

// longTappableStyled is longTappable with styles for its own box
// (FlexGrow, to share a row).
func longTappableStyled(scope, key any, st giode.Styles, onClick, onLong func(), content func(s giode.PressState) giode.Element) giode.Element {
	p := giode.Keyed(scope, key, func() *giode.PressableElement {
		return giode.Pressable(content, st)
	})
	p.Content(content).
		OnClick(func(giode.ClickEvent) { onClick() }).
		OnLongPress(func(giode.ClickEvent) { onLong() }).
		OnSecondaryClick(func(giode.ClickEvent) { onLong() })
	return p
}

// pressScale shrinks pressed content slightly, for tactile feedback.
func pressScale(s giode.PressState) giode.Transform {
	if s.Pressed {
		return giode.Transform{Scale: 0.96}
	}
	return giode.Transform{}
}
