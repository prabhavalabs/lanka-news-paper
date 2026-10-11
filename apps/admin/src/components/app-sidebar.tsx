import * as React from 'react'
import {
  CalendarDaysIcon,
  FeatherIcon,
  LayoutTemplateIcon,
  LayoutDashboardIcon,
  ListTreeIcon,
  ListChecksIcon,
  MailIcon,
  Share2Icon,
  MessageSquareWarningIcon,
  NetworkIcon,
  NewspaperIcon,
  PaletteIcon,
  RadioTowerIcon,
  SettingsIcon,
  SparklesIcon,
  TelescopeIcon,
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
  { title: 'Articles', url: '/articles', icon: NewspaperIcon },
  { title: 'Knowledge graph', url: '/knowledge', icon: NetworkIcon },
  { title: 'Watch Tower', url: '/watch-tower', icon: TelescopeIcon },
  { title: 'Queue monitor', url: '/jobs', icon: ListTreeIcon },
  { title: 'Editorial queue', url: '/queue', icon: ListChecksIcon },
  { title: 'Complaints', url: '/complaints', icon: MessageSquareWarningIcon },
]

// The chief editor curates the public front page and writes editorials; administrators can too.
const curatorNavigation: NavItem[] = [
  { title: 'Front page', url: '/front-page', icon: LayoutTemplateIcon },
  { title: 'Editorials', url: '/editorials', icon: FeatherIcon },
]

const administratorNavigation: NavItem[] = [
  { title: 'Workflow manager', url: '/workflows', icon: SparklesIcon },
  { title: 'Mailing list', url: '/mailing-list', icon: MailIcon },
  { title: 'Social publishing', url: '/social', icon: Share2Icon },
  { title: 'Content calendar', url: '/social/content', icon: CalendarDaysIcon },
  { title: 'Brand kit', url: '/brand', icon: PaletteIcon },
  { title: 'Settings', url: '/settings', icon: SettingsIcon },
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
              tooltip="Thulawa Control Room"
              className="data-[slot=sidebar-menu-button]:p-1.5!"
            >
              <img src="/brand/icon-192.png" alt="" className="size-7 rounded-md" />
              <span className="text-base font-semibold tracking-tight">Thulawa Control Room</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <NavMain items={[
          ...navigation,
          ...(user.role === 'administrator' || user.role === 'chief_editor' ? curatorNavigation : []),
          ...(user.role === 'administrator' ? administratorNavigation : []),
        ]} />
      </SidebarContent>
      <SidebarFooter>
        <NavUser user={user} onLogout={onLogout} />
      </SidebarFooter>
    </Sidebar>
  )
}
