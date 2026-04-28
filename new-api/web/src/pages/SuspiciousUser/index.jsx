import React, { useEffect, useState } from 'react';
import { Button, Card, DatePicker, Empty, Space, Table, Tag, Typography } from '@douyinfe/semi-ui';
import { IllustrationNoResult, IllustrationNoResultDark } from '@douyinfe/semi-illustrations';
import { useTranslation } from 'react-i18next';
import { API, showError, timestamp2string } from '../../helpers';

const { Text } = Typography;

const SuspiciousUser = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState([]);
  const [range, setRange] = useState([]);

  const loadData = async () => {
    setLoading(true);
    try {
      let url = '/api/log/suspicious?limit=100&min_score=1';
      if (Array.isArray(range) && range.length === 2 && range[0] && range[1]) {
        const start = Math.floor(new Date(range[0]).getTime() / 1000);
        const end = Math.floor(new Date(range[1]).getTime() / 1000);
        url += `&start_timestamp=${start}&end_timestamp=${end}`;
      }
      const res = await API.get(url, { disableDuplicate: true });
      if (res.data.success) {
        setData(res.data.data || []);
      } else {
        showError(res.data.message || t('加载失败'));
      }
    } catch (error) {
      showError(error.message || t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const columns = [
    {
      title: t('用户'),
      dataIndex: 'username',
      render: (text, record) => (
        <Space vertical align='start' spacing={2}>
          <Text strong>{text || '-'}</Text>
          <Text type='tertiary'>ID: {record.user_id}</Text>
        </Space>
      ),
    },
    {
      title: t('风险分'),
      dataIndex: 'score',
      sorter: (a, b) => a.score - b.score,
      render: (score) => <Tag color={score >= 5 ? 'red' : score >= 3 ? 'orange' : 'yellow'}>{score}</Tag>,
    },
    {
      title: t('原因'),
      dataIndex: 'reasons',
      render: (reasons) => (
        <Space wrap>
          {(reasons || []).map((reason) => (
            <Tag key={reason} color='red'>{reason}</Tag>
          ))}
        </Space>
      ),
    },
    {
      title: t('请求数'),
      dataIndex: 'request_count',
      sorter: (a, b) => a.request_count - b.request_count,
    },
    {
      title: t('IP'),
      dataIndex: 'ips',
      render: (ips, record) => (
        <Space vertical align='start' spacing={2}>
          <Text>{record.ip_count}</Text>
          <Text type='tertiary' style={{ maxWidth: 260, wordBreak: 'break-word' }}>
            {(ips || []).join(', ')}
          </Text>
        </Space>
      ),
    },
    {
      title: t('活跃小时'),
      dataIndex: 'active_hour_count',
      sorter: (a, b) => a.active_hour_count - b.active_hour_count,
    },
    {
      title: t('接近限速小时'),
      dataIndex: 'near_limit_hour_count',
      sorter: (a, b) => a.near_limit_hour_count - b.near_limit_hour_count,
    },
    {
      title: t('客户端样本'),
      dataIndex: 'sample_client',
      render: (text) => <Text style={{ maxWidth: 280, wordBreak: 'break-word' }}>{text || '-'}</Text>,
    },
    {
      title: t('时间范围'),
      dataIndex: 'first_seen',
      render: (_, record) => (
        <Space vertical align='start' spacing={2}>
          <Text>{timestamp2string(record.first_seen)}</Text>
          <Text type='tertiary'>{timestamp2string(record.last_seen)}</Text>
        </Space>
      ),
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <Card className='rounded-xl'>
        <Space className='mb-4' wrap>
          <DatePicker type='dateTimeRange' value={range} onChange={setRange} />
          <Button type='primary' onClick={loadData} loading={loading}>{t('刷新')}</Button>
          <Text type='tertiary'>{t('基于最近消费日志实时聚合，仅用于辅助人工审判。')}</Text>
        </Space>
        <Table
          columns={columns}
          dataSource={data}
          rowKey='user_id'
          loading={loading}
          size='small'
          pagination={{ pageSize: 20 }}
          empty={
            <Empty
              image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
              darkModeImage={<IllustrationNoResultDark style={{ width: 150, height: 150 }} />}
              description={t('暂无可疑用户')}
              style={{ padding: 30 }}
            />
          }
        />
      </Card>
    </div>
  );
};

export default SuspiciousUser;
