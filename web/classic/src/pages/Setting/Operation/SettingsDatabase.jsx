/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useRef, useState } from 'react';
import { Button, Form, Space, Typography, Modal } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../helpers';

const { Text } = Typography;

export default function SettingsDatabase() {
  const { t } = useTranslation();
  const fileRef = useRef(null);
  const [loading, setLoading] = useState(false);

  const exportDatabase = () => {
    window.open('/api/database/export', '_blank');
  };

  const importDatabase = async () => {
    const file = fileRef.current?.files?.[0];
    if (!file) {
      showError(t('请选择数据库文件'));
      return;
    }
    const formData = new FormData();
    formData.append('file', file);
    setLoading(true);
    try {
      const res = await API.post('/api/database/import', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('数据库导入成功，请点击重启 New API 生效'));
      } else {
        showError(message || t('数据库导入失败'));
      }
    } catch (error) {
      showError(error.message || t('数据库导入失败'));
    } finally {
      setLoading(false);
    }
  };

  const restartService = () => {
    Modal.confirm({
      title: t('确认重启 New API？'),
      content: t('服务会短暂中断，Docker 会自动重新拉起容器。'),
      okText: t('确认重启'),
      cancelText: t('取消'),
      onOk: async () => {
        try {
          await API.post('/api/database/restart');
          showSuccess(t('重启请求已发送，请等待数秒后刷新页面'));
        } catch (error) {
          showError(error.message || t('重启请求失败'));
        }
      },
    });
  };

  return (
    <Form.Section text={t('数据库导入导出')}>
      <Text type='tertiary'>
        {t('当前 SQLite 数据库可直接导出备份，也可以上传数据库文件导入。导入后需要重启 New API。')}
      </Text>
      <div style={{ marginTop: 16 }}>
        <input ref={fileRef} type='file' accept='.db,.sqlite,.sqlite3' />
      </div>
      <Space style={{ marginTop: 16 }} wrap>
        <Button onClick={exportDatabase}>{t('导出数据库')}</Button>
        <Button type='warning' loading={loading} onClick={importDatabase}>
          {t('导入数据库')}
        </Button>
        <Button type='danger' onClick={restartService}>
          {t('重启 New API')}
        </Button>
      </Space>
    </Form.Section>
  );
}
