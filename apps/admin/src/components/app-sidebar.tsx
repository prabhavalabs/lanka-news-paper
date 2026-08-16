import * as React from 'react'
import {
  LayoutDashboardIcon,
  ListChecksIcon,
  MessageSquareWarningIcon,
  NewspaperIcon,
  RadioTowerIcon,
  SparklesIcon,
} from 'lucide-react'
import { Link } from 'react-router'

import { NavMain, type NavItem } from '@/components/nav-main'
import { NavUser } from '@/components/nav-user'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '@/components/ui/sidebar'

const navigation: NavItem[] = [
  { title: 'Overview', url: '/', icon: LayoutDashboardIcon },
  { title: 'Sources', url: '/sources', icon: RadioTowerIcon },
  { title: 'Editorial queue', url: '/queue', icon: ListChecksIcon },
  { title: 'Complaints', url: '/complaints', icon: MessageSquareWarningIcon },
  { title: 'AI & routing', url: '/routing', icon: SparklesIcon },
]

type AppSidebarProps = React.ComponentProps<typeof Sidebar> & {
  user: { email: string; name: string; role: string }
  onLogout: () => void | Promise<void>
}

export function AppSidebar({ user, onLogout, ...props }: AppSidebarProps) {
  return (
    <Sidebar collapsible="icon" {...props}>
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              render={<Link to="/" />}
              tooltip="SNAP newsroom"
              className="data-[slot=sidebar-menu-button]:p-1.5!"
            >
              <span className="flex size-7 items-center justify-center rounded-lg bg-primary text-primary-foreground">
                <NewspaperIcon className="size-4" />
              </span>
              <span className="text-base font-semibold tracking-tight">SNAP newsroom</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <NavMain items={navigation} />
      </SidebarContent>
      <SidebarFooter>
        <NavUser user={user} onLogout={onLogout} />
      </SidebarFooter>
    </Sidebar>
  )
}
