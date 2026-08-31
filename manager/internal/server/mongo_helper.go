package server

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoHelper MongoDB 辅助函数集合
type MongoHelper struct {
	db *mongo.Database
}

// NewMongoHelper 创建 MongoDB 辅助工具
func NewMongoHelper(db *mongo.Database) *MongoHelper {
	return &MongoHelper{db: db}
}

// FindOne 查询单条记录
func (h *MongoHelper) FindOne(collection string, filter interface{}, result interface{}) error {
	ctx := context.Background()
	return h.db.Collection(collection).FindOne(ctx, filter).Decode(result)
}

// Find 查询多条记录
func (h *MongoHelper) Find(collection string, filter interface{}, results interface{}, opts ...*options.FindOptions) error {
	ctx := context.Background()
	cursor, err := h.db.Collection(collection).Find(ctx, filter, opts...)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)
	return cursor.All(ctx, results)
}

// InsertOne 插入单条记录
func (h *MongoHelper) InsertOne(collection string, document interface{}) (interface{}, error) {
	ctx := context.Background()
	result, err := h.db.Collection(collection).InsertOne(ctx, document)
	if err != nil {
		return nil, err
	}
	return result.InsertedID, nil
}

// UpdateOne 更新单条记录
func (h *MongoHelper) UpdateOne(collection string, filter interface{}, update interface{}) error {
	ctx := context.Background()
	_, err := h.db.Collection(collection).UpdateOne(ctx, filter, update)
	return err
}

// DeleteOne 删除单条记录（硬删除）
func (h *MongoHelper) DeleteOne(collection string, filter interface{}) error {
	ctx := context.Background()
	_, err := h.db.Collection(collection).DeleteOne(ctx, filter)
	return err
}

// SoftDelete 软删除（设置 deleted_at 字段）
func (h *MongoHelper) SoftDelete(collection string, filter interface{}) error {
	ctx := context.Background()
	now := time.Now()
	update := bson.M{"$set": bson.M{"deleted_at": &now, "updated_at": now}}
	_, err := h.db.Collection(collection).UpdateOne(ctx, filter, update)
	return err
}

// Count 统计记录数
func (h *MongoHelper) Count(collection string, filter interface{}) (int64, error) {
	ctx := context.Background()
	return h.db.Collection(collection).CountDocuments(ctx, filter)
}

// Exists 检查记录是否存在
func (h *MongoHelper) Exists(collection string, filter interface{}) (bool, error) {
	count, err := h.Count(collection, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// FindByID 根据 ObjectID 查询
func (h *MongoHelper) FindByID(collection string, id string, result interface{}) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	filter := bson.M{"_id": objectID, "deleted_at": nil}
	return h.FindOne(collection, filter, result)
}

// UpdateByID 根据 ObjectID 更新
func (h *MongoHelper) UpdateByID(collection string, id string, update interface{}) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	filter := bson.M{"_id": objectID}
	return h.UpdateOne(collection, filter, update)
}

// DeleteByID 根据 ObjectID 删除（软删除）
func (h *MongoHelper) DeleteByID(collection string, id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	filter := bson.M{"_id": objectID}
	return h.SoftDelete(collection, filter)
}

// Paginate 分页查询
func (h *MongoHelper) Paginate(collection string, filter interface{}, page, pageSize int, results interface{}) (int64, error) {
	ctx := context.Background()

	// 获取总数
	total, err := h.Count(collection, filter)
	if err != nil {
		return 0, err
	}

	// 分页查询
	skip := int64((page - 1) * pageSize)
	limit := int64(pageSize)
	opts := options.Find().SetSkip(skip).SetLimit(limit)

	cursor, err := h.db.Collection(collection).Find(ctx, filter, opts)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, results); err != nil {
		return 0, err
	}

	return total, nil
}

// Aggregate 聚合查询
func (h *MongoHelper) Aggregate(collection string, pipeline interface{}, results interface{}) error {
	ctx := context.Background()
	cursor, err := h.db.Collection(collection).Aggregate(ctx, pipeline)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)
	return cursor.All(ctx, results)
}

// BulkWrite 批量写入
func (h *MongoHelper) BulkWrite(collection string, operations []mongo.WriteModel) error {
	ctx := context.Background()
	_, err := h.db.Collection(collection).BulkWrite(ctx, operations)
	return err
}

// Upsert 插入或更新（基于唯一字段）
func (h *MongoHelper) Upsert(collection string, filter interface{}, update interface{}) error {
	ctx := context.Background()
	opts := options.Update().SetUpsert(true)
	_, err := h.db.Collection(collection).UpdateOne(ctx, filter, update, opts)
	return err
}

// FindWithSort 查询并排序
func (h *MongoHelper) FindWithSort(collection string, filter interface{}, sort interface{}, results interface{}) error {
	ctx := context.Background()
	opts := options.Find().SetSort(sort)
	cursor, err := h.db.Collection(collection).Find(ctx, filter, opts)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)
	return cursor.All(ctx, results)
}

// FindOneAndUpdate 查询并更新
func (h *MongoHelper) FindOneAndUpdate(collection string, filter interface{}, update interface{}, result interface{}) error {
	ctx := context.Background()
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	return h.db.Collection(collection).FindOneAndUpdate(ctx, filter, update, opts).Decode(result)
}

// Transaction 执行事务
func (h *MongoHelper) Transaction(fn func(sessCtx mongo.SessionContext) error) error {
	ctx := context.Background()
	session, err := h.db.Client().StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		return nil, fn(sessCtx)
	})
	return err
}

// BuildFilter 构建查询过滤器（排除已删除记录）
func BuildFilter(conditions map[string]interface{}) bson.M {
	filter := bson.M{"deleted_at": nil}
	for k, v := range conditions {
		filter[k] = v
	}
	return filter
}

// BuildUpdate 构建更新操作（自动添加 updated_at）
func BuildUpdate(updates map[string]interface{}) bson.M {
	updates["updated_at"] = time.Now()
	return bson.M{"$set": updates}
}

// StringToObjectID 转换字符串为 ObjectID
func StringToObjectID(id string) (primitive.ObjectID, error) {
	return primitive.ObjectIDFromHex(id)
}

// StringsToObjectIDs 转换字符串数组为 ObjectID 数组
func StringsToObjectIDs(ids []string) ([]primitive.ObjectID, error) {
	objectIDs := make([]primitive.ObjectID, len(ids))
	for i, id := range ids {
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return nil, err
		}
		objectIDs[i] = oid
	}
	return objectIDs, nil
}

// ObjectIDToString 转换 ObjectID 为字符串
func ObjectIDToString(id interface{}) string {
	if oid, ok := id.(primitive.ObjectID); ok {
		return oid.Hex()
	}
	if str, ok := id.(string); ok {
		return str
	}
	return ""
}
