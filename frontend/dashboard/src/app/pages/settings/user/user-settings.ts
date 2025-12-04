import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ButtonModule } from 'primeng/button';
import { SettingsSecurity } from '../../../features/settings/components/settings-security';

@Component({
  selector: 'app-user-settings',
  standalone: true,
  imports: [CommonModule, SettingsSecurity, ButtonModule],
  templateUrl: './user-settings.html'
})
export class UserSettings {
  downloadData() {
    window.open('/api/user/takeout', '_blank');
  }
}