import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SettingsSecurity } from '../../features/settings/components/settings-security';

@Component({
  selector: 'app-settings',
  standalone: true,
  imports: [CommonModule, SettingsSecurity],
  templateUrl: './settings.html'
})
export class Settings {
}
