import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'ops-client-downloads-page',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './client-downloads-page.component.html',
})
// 客户端发布页面，负责展示和提交各平台安装包版本、渠道与下载地址。
export class ClientDownloadsPageComponent {
  // 根组件下发的共享视图模型，包含发布包列表、表单和保存/删除方法。
  @Input({ required: true }) vm!: any;
}
