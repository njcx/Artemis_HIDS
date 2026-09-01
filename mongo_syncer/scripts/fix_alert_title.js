// MongoDB 脚本：修复告警标题缺失问题
// 将 rule_name 字段的值复制到 title 字段
// 
// 使用方法：
// mongosh "mongodb://localhost:27017/hids" fix_alert_title.js
// 或者
// mongo "mongodb://localhost:27017/hids" fix_alert_title.js

// 查找所有没有 title 字段但有 rule_name 字段的告警
const result = db.agent_security_events.updateMany(
  {
    $or: [
      { title: { $exists: false } },
      { title: null },
      { title: "" }
    ],
    rule_name: { $exists: true, $ne: null, $ne: "" }
  },
  [
    {
      $set: {
        title: "$rule_name"
      }
    }
  ]
);

print("更新结果:");
print("  匹配的文档数: " + result.matchedCount);
print("  修改的文档数: " + result.modifiedCount);

// 验证更新结果
const emptyTitleCount = db.agent_security_events.countDocuments({
  $or: [
    { title: { $exists: false } },
    { title: null },
    { title: "" }
  ]
});

print("\n验证:");
print("  仍然没有 title 的告警数: " + emptyTitleCount);

if (emptyTitleCount > 0) {
  print("\n警告: 仍有 " + emptyTitleCount + " 条告警没有 title 字段");
  print("这些告警可能也没有 rule_name 字段，需要手动检查");
  
  // 显示一些示例
  const samples = db.agent_security_events.find({
    $or: [
      { title: { $exists: false } },
      { title: null },
      { title: "" }
    ]
  }).limit(5).toArray();
  
  print("\n示例告警:");
  samples.forEach(function(doc) {
    print("  ID: " + doc._id + ", rule_name: " + (doc.rule_name || "无") + ", event_type: " + (doc.event_type || "无"));
  });
}
