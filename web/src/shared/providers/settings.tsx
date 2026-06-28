import React, { createContext, useCallback, useContext } from 'react'
import { UsedStorageKeys, useStorage } from '~/shared'

type Settings = {
  // client-side settings:
  showRequestDetails: boolean
  autoNavigateToNewRequest: boolean
  showNativeRequestNotifications: boolean
  uiScale: number // interface scale factor (1 = 100%), applied via the Mantine scale CSS variable
  // server-side setting:
  maxRequestsPerSession: number | null
  maxRequestBodySize: number | null
  sessionTTLSec: number | null
  tunnelEnabled: boolean | null
  tunnelUrl: URL | null
  publicUrlRoot: URL | null
}

type SettingsContext = Settings & {
  updateSettings(newSettings: Partial<Readonly<Settings>>): void
}

const notInitialized = (): never => {
  throw new Error('The SettingsProvider is not initialized')
}

const defaults: Readonly<Settings> = {
  showRequestDetails: true,
  autoNavigateToNewRequest: true,
  showNativeRequestNotifications: false,
  uiScale: 1,
  maxRequestsPerSession: null,
  maxRequestBodySize: null,
  sessionTTLSec: null,
  tunnelEnabled: null,
  tunnelUrl: null,
  publicUrlRoot: null,
}

const uiSettingsContext = createContext<SettingsContext>({
  ...defaults,
  updateSettings: () => notInitialized(),
})

export const SettingsProvider = ({ children }: { children: React.JSX.Element }) => {
  const [settings, setSettings] = useStorage<Settings>(defaults, UsedStorageKeys.UISettings, 'local')
  const updateSettings = useCallback(
    (upd: Partial<Readonly<Settings>>) => setSettings((prev) => ({ ...prev, ...upd })),
    [setSettings]
  )

  return (
    <uiSettingsContext.Provider
      value={{
        ...defaults, // ensure newly-added fields fall back to defaults for older stored settings
        ...settings,
        updateSettings,
      }}
    >
      {children}
    </uiSettingsContext.Provider>
  )
}

export function useSettings(): SettingsContext {
  const ctx = useContext(uiSettingsContext)

  if (!ctx) {
    throw new Error('useSettings must be used within a SettingsProvider')
  }

  return ctx
}
