import React, { useEffect, useState } from 'react'
import { CodeHighlight } from '@mantine/code-highlight'
import { Badge, Button, Flex, Grid, Skeleton, Table, Tabs, Text, Title } from '@mantine/core'
import { useInterval } from '@mantine/hooks'
import { Link } from 'react-router-dom'
import { IconBinary, IconDownload, IconLetterCase } from '@tabler/icons-react'
import dayjs from 'dayjs'
import { useData, UsedStorageKeys, useSettings, useStorage } from '~/shared'
import { methodToColor } from '~/theme'
import { ViewHex, ViewText } from './components'

export const RequestDetails: React.FC<{ loading?: boolean }> = ({ loading = false }) => {
  const { session, request, webHookUrl } = useData()
  const { showRequestDetails } = useSettings()

  const [headersExpanded, setHeadersExpanded] = useStorage<boolean>(false, UsedStorageKeys.RequestDetailsHeadersExpand)
  const [elapsedTime, setElapsedTime] = useState<string | null>(null)
  const [contentType, setContentType] = useState<string | null>(null)
  const [payload, setPayload] = useState<Uint8Array | null>(null)

  useEffect(
    () => setContentType(request?.headers.find(({ name }) => name.toLowerCase() === 'content-type')?.value ?? null),
    [request]
  )

  // automatically update the payload
  useEffect(() => {
    request?.payload?.then((data) => setPayload(data))
  }, [request, request?.payload])

  // automatically update the elapsed time
  useEffect(
    () => setElapsedTime(request?.capturedAt ? dayjs(request?.capturedAt).fromNow() : null),
    [request?.capturedAt, setElapsedTime]
  )
  const interval = useInterval(
    () => setElapsedTime(request?.capturedAt ? dayjs(request?.capturedAt).fromNow() : null),
    1000
  )

  useEffect((): (() => void) => {
    interval.start()

    return interval.stop // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <Grid>
      {!!request && !!session && showRequestDetails && (
        <>
          <Grid.Col span={{ base: 12, md: 6 }}>
            <Title order={4} mb="md">
              Request details
            </Title>
            <Table my="md" withRowBorders={false} verticalSpacing="0.2em" highlightOnHover>
              <Table.Tbody>
                <Table.Tr>
                  <Table.Td ta="right" w="15%">
                    Path
                  </Table.Td>
                  <Table.Td>
                    {(loading && <Skeleton radius="xl" h="sm" w="80%" />) ||
                      (request.url && <WebHookPath sID={session.sID} url={request.url} />) || <>...</>}
                  </Table.Td>
                </Table.Tr>
                <Table.Tr>
                  <Table.Td ta="right">Method</Table.Td>
                  <Table.Td>
                    {(loading && <Skeleton radius="xl" h="sm" w="15%" />) || (
                      <Badge color={methodToColor(request.method ?? '')} mb="0.2em">
                        {request.method}
                      </Badge>
                    )}
                  </Table.Td>
                </Table.Tr>
                <Table.Tr>
                  <Table.Td ta="right">From</Table.Td>
                  <Table.Td>
                    {(loading && <Skeleton radius="xl" h="sm" w="20%" />) || (
                      <Flex justify="flex-start" align="center">
                        <Text span>{request.clientAddress}</Text>
                        <Flex align="center" ml="md" gap="sm">
                          {[
                            ['WhoIs', 'https://who.is/whois-ip/ip-address/' + request.clientAddress],
                            ['Shodan', 'https://www.shodan.io/host/' + request.clientAddress],
                            ['Netify', 'https://www.netify.ai/resources/ips/' + request.clientAddress],
                            ['Censys', 'https://search.censys.io/hosts/' + request.clientAddress],
                          ].map(([name, link], index) => (
                            <Link key={index} to={link} target="_blank" rel="noreferrer">
                              {name}
                            </Link>
                          ))}
                        </Flex>
                      </Flex>
                    )}
                  </Table.Td>
                </Table.Tr>
                <Table.Tr>
                  <Table.Td ta="right">When</Table.Td>
                  <Table.Td>
                    {(loading && <Skeleton radius="xl" h="sm" w="45%" />) || (
                      <>
                        {request.capturedAt && <>{dayjs(request.capturedAt).format('YYYY-MM-DD HH:mm:ss.SSS')}</>}
                        {elapsedTime && <span style={{ paddingLeft: '0.3em' }}>({elapsedTime})</span>}
                      </>
                    )}
                  </Table.Td>
                </Table.Tr>
                <Table.Tr>
                  <Table.Td ta="right">Size</Table.Td>
                  <Table.Td>
                    {(loading && <Skeleton radius="xl" h="sm" w="15%" />) || <>{payload?.length} bytes</>}
                  </Table.Td>
                </Table.Tr>
                <Table.Tr>
                  <Table.Td ta="right">
                    <Text size="xs" c="dimmed" span>
                      ID
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    {(loading && <Skeleton radius="xl" h="xs" w="50%" />) || (
                      <Text size="xs" c="dimmed" span>
                        {request.rID}
                      </Text>
                    )}
                  </Table.Td>
                </Table.Tr>
              </Table.Tbody>
            </Table>
          </Grid.Col>
          <Grid.Col span={{ base: 12, md: 6 }}>
            <Title order={4} mb="md">
              HTTP headers
            </Title>
            {(loading && <Skeleton radius="md" h="10em" w="100%" />) ||
              (!!request.headers && (
                <CodeHighlight
                  code={request.headers.map(({ name, value }) => `${name}: ${value}`).join('\n')}
                  language="bash"
                  expandCodeLabel="Show all headers"
                  maxCollapsedHeight="10em"
                  expanded={headersExpanded}
                  onExpandedChange={setHeadersExpanded}
                  withExpandButton
                  withCopyButton
                />
              ))}
          </Grid.Col>
        </>
      )}

      {!loading && !!request && request.files.length > 0 && (
        <Grid.Col span={12}>
          <Title order={4} mb="md">
            Files
            <Text span c="dimmed" size="sm" ml="xs">
              ({request.files.length})
            </Text>
          </Title>
          <Text size="xs" c="dimmed" mb="sm">
            Files extracted from the multipart/form-data body. They are stored on the server and served only through
            this app.
          </Text>
          <Table withTableBorder withRowBorders verticalSpacing="xs" highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Name</Table.Th>
                <Table.Th>Type</Table.Th>
                <Table.Th>Size</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {request.files.map((file) => (
                <Table.Tr key={file.uuid}>
                  <Table.Td style={{ wordBreak: 'break-all' }}>{file.name}</Table.Td>
                  <Table.Td>
                    <Text size="sm" c="dimmed" span>
                      {file.contentType || 'application/octet-stream'}
                    </Text>
                  </Table.Td>
                  <Table.Td>{formatBytes(file.size)}</Table.Td>
                  <Table.Td ta="right">
                    <Button
                      variant="light"
                      color="indigo"
                      size="compact-sm"
                      leftSection={<IconDownload size="1.2em" />}
                      component="a"
                      href={fileDownloadUrl(webHookUrl, file.uuid)}
                      download={file.name}
                      disabled={!webHookUrl}
                    >
                      Download
                    </Button>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Grid.Col>
      )}

      <Grid.Col span={12}>
        <Title order={4} mb="md">
          Request body
          {!loading && !!request && !!payload && payload.length > 0 && (
            <Button
              variant="light"
              color="indigo"
              size="compact-sm"
              ml="sm"
              leftSection={<IconDownload size="1.2em" />}
              onClick={() => (payload ? download(payload, `${request.rID}.bin`) : undefined)}
            >
              Download
            </Button>
          )}
        </Title>
        {(loading && <Skeleton radius="md" h="8em" w="100%" />) || (
          <Tabs variant="default" defaultValue={TabsList.Text} keepMounted={false}>
            <Tabs.List>
              <Tabs.Tab value={TabsList.Text} leftSection={<IconLetterCase />} color="blue">
                Text
              </Tabs.Tab>
              {!!payload && payload.length > 0 && (
                <Tabs.Tab value={TabsList.Binary} leftSection={<IconBinary />} color="teal">
                  Binary
                </Tabs.Tab>
              )}
            </Tabs.List>
            <Tabs.Panel value={TabsList.Text}>
              <ViewText input={payload || null} contentType={contentType} />
            </Tabs.Panel>
            {!!payload && payload.length > 0 && (
              <Tabs.Panel value={TabsList.Binary}>
                <ViewHex input={payload} />
              </Tabs.Panel>
            )}
          </Tabs>
        )}
      </Grid.Col>
    </Grid>
  )
}

enum TabsList {
  Text = 'Text',
  Binary = 'Binary',
}
export const WebHookPath: React.FC<{ sID: string; url: URL }> = ({ sID, url }) => {
  const { search, hash } = url // search may be '', '?' or '?key=value'; hash may be '', '#' or '#fragment'
  let { pathname } = url // pathname is usually '/{sID}' or '/{sID}/any/path'

  // remove the sID from the pathname since it's already displayed and useless a bit
  if (pathname.startsWith('/' + sID)) {
    pathname = pathname.slice(sID.length + 1)
  }

  // if the pathname is empty, set it to '/'
  if (pathname === '') {
    pathname = '/'
  }

  return (
    <Text size="md" style={{ wordBreak: 'break-all' }}>
      <Text span>{pathname}</Text>
      {search && (
        <Text variant="gradient" gradient={{ from: 'yellow', to: 'orange', deg: 90 }} span>
          {search}
        </Text>
      )}
      {hash && <Text c="dimmed">{hash}</Text>}
      <Button
        variant="light"
        color="gray"
        size="compact-xs"
        component="a"
        href={`${url.pathname}${search}${hash}`}
        target="_blank"
        ml="sm"
        mb="0.1em"
      >
        Open
      </Button>
    </Text>
  )
}

/** Builds the in-app download URL for a stored file: {webhook-url}/files/{file-uuid}. */
const fileDownloadUrl = (webHookUrl: Readonly<URL> | null, fileUUID: string): string => {
  if (!webHookUrl) {
    return '#'
  }

  const u = new URL(webHookUrl.toString())
  u.pathname = `${u.pathname.replace(/\/+$/, '')}/files/${encodeURIComponent(fileUUID)}`

  return u.toString()
}

/** Formats a byte count into a human-readable string. */
const formatBytes = (bytes: number): string => {
  if (bytes < 1024) {
    return `${bytes} B`
  }

  const units = ['KiB', 'MiB', 'GiB', 'TiB']
  let value = bytes / 1024
  let unitIndex = 0

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex++
  }

  return `${value.toFixed(1)} ${units[unitIndex]}`
}

const download = (data: Readonly<Uint8Array>, fileName: string): void => {
  const blob = new Blob([new Uint8Array(data)], { type: 'application/octet-stream' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')

  a.href = url
  a.download = fileName
  a.click()

  setTimeout(() => {
    URL.revokeObjectURL(url)

    a.remove()
  }, 1000) // 1s
}
