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
import { Banner, Button, Card, Form, Modal, Space } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import SecureVerificationModal from '../common/modals/SecureVerificationModal';
import { useSecureVerification } from '../../hooks/common/useSecureVerification';

const DatabaseBackupCard = () => {
  const { t } = useTranslation();
  const [databaseBackupLoading, setDatabaseBackupLoading] = useState(false);
  const [databaseImportLoading, setDatabaseImportLoading] = useState(false);
  const importInputRef = useRef(null);

  const downloadBlob = (blob, filename) => {
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  const downloadDatabaseBackup = async () => {
    try {
      const res = await API.get('/api/performance/database_backup', {
        responseType: 'blob',
        disableDuplicate: true,
        skipErrorHandler: true,
      });
      const disposition = res.headers?.['content-disposition'] || '';
      const filenameMatch = disposition.match(/filename="?([^";]+)"?/i);
      const filename =
        filenameMatch?.[1] || `new-api-sqlite-backup-${Date.now()}.db`;
      downloadBlob(res.data, filename);
      return { success: true };
    } finally {
      setDatabaseBackupLoading(false);
    }
  };

  const importDatabaseBackup = async (file) => {
    try {
      const formData = new FormData();
      formData.append('database', file);
      const res = await API.post('/api/performance/database_backup/import', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
        disableDuplicate: true,
        skipErrorHandler: true,
      });
      if (!res.data?.success) {
        throw new Error(res.data?.message || t('数据库导入失败'));
      }
      return res.data;
    } finally {
      setDatabaseImportLoading(false);
      if (importInputRef.current) {
        importInputRef.current.value = '';
      }
    }
  };

  const {
    isModalVisible,
    verificationMethods,
    verificationState,
    startVerification,
    executeVerification,
    cancelVerification,
    setVerificationCode,
    switchVerificationMethod,
  } = useSecureVerification({
    successMessage: t('数据库备份已开始下载'),
  });

  const exportDatabaseBackup = async () => {
    Modal.confirm({
      title: t('导出 SQLite 数据库备份'),
      content: t(
        '数据库备份包含用户、令牌、日志、充值等敏感数据。请确认当前设备安全，下载后妥善保存，不要上传到公开仓库或聊天工具。',
      ),
      okText: t('继续导出'),
      cancelText: t('取消'),
      okButtonProps: { type: 'danger' },
      onOk: async () => {
        setDatabaseBackupLoading(true);
        try {
          const started = await startVerification(downloadDatabaseBackup, {
            title: t('导出数据库备份'),
            description: t(
              '为了保护数据库安全，请先完成两步验证或 Passkey 验证。',
            ),
            preferredMethod: 'passkey',
          });
          if (!started) {
            setDatabaseBackupLoading(false);
          }
        } catch (error) {
          setDatabaseBackupLoading(false);
          showError(error.message || t('数据库备份导出失败'));
        }
      },
    });
  };

  const confirmImportDatabaseBackup = (file) => {
    Modal.confirm({
      title: t('导入 SQLite 数据库备份'),
      content: t(
        '导入会替换当前 SQLite 数据库，当前数据库会先保存为 .before-import-时间.bak。导入成功后服务会自动停止，你需要手动重启服务后新数据库才会生效。请确认已经下载并保存当前数据库备份。',
      ),
      okText: t('确认导入并停止服务'),
      cancelText: t('取消'),
      okButtonProps: { type: 'danger' },
      onOk: async () => {
        setDatabaseImportLoading(true);
        try {
          const started = await startVerification(
            () => importDatabaseBackup(file),
            {
              title: t('导入数据库备份'),
              description: t(
                '导入数据库是高危操作。为了保护数据库安全，请先完成两步验证或 Passkey 验证。',
              ),
              preferredMethod: 'passkey',
            },
          );
          if (!started) {
            setDatabaseImportLoading(false);
          }
        } catch (error) {
          setDatabaseImportLoading(false);
          showError(error.message || t('数据库导入失败'));
        }
      },
      onCancel: () => {
        if (importInputRef.current) {
          importInputRef.current.value = '';
        }
      },
    });
  };

  const handleImportFileChange = (event) => {
    const file = event.target.files?.[0];
    if (!file) return;
    confirmImportDatabaseBackup(file);
  };

  return (
    <Card>
      <Form.Section text={t('数据库备份')}>
        <Banner
          type='warning'
          description={t(
            '仅支持 SQLite 主数据库导出。备份文件包含完整业务数据和敏感信息，下载后请离线妥善保存，不要提交到 Git 或公开分享。',
          )}
          style={{ marginBottom: 16 }}
        />
        <Space wrap>
          <Button
            type='danger'
            loading={databaseBackupLoading}
            onClick={exportDatabaseBackup}
          >
            {t('导出 SQLite 数据库')}
          </Button>
          <Button
            type='warning'
            loading={databaseImportLoading}
            onClick={() => importInputRef.current?.click()}
          >
            {t('导入 SQLite 数据库')}
          </Button>
        </Space>
        <input
          ref={importInputRef}
          type='file'
          accept='.db,.sqlite,.sqlite3,application/vnd.sqlite3,application/octet-stream'
          style={{ display: 'none' }}
          onChange={handleImportFileChange}
        />
      </Form.Section>

      <SecureVerificationModal
        visible={isModalVisible}
        verificationMethods={verificationMethods}
        verificationState={verificationState}
        onVerify={executeVerification}
        onCancel={cancelVerification}
        onCodeChange={setVerificationCode}
        onMethodSwitch={switchVerificationMethod}
        title={verificationState.title || t('导出数据库备份')}
        description={verificationState.description}
      />
    </Card>
  );
};

export default DatabaseBackupCard;
