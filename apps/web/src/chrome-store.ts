import { create } from 'zustand'

type ChromeState = {
  condensed: boolean
  searchOpen: boolean
  setCondensed: (condensed: boolean) => void
  setSearchOpen: (searchOpen: boolean) => void
}

export const useChromeStore = create<ChromeState>()((set) => ({
  condensed: false,
  searchOpen: false,
  setCondensed: (condensed) => set({ condensed }),
  setSearchOpen: (searchOpen) => set({ searchOpen }),
}))
