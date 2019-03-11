# golang + postgreSQL practice

## 题目
两个用户匹配（liked, disliked, matched）

## 思路
通过 PostgreSQL 数据库层面防止并发

## 表结构
- user_info (用户信息表)   
```
   Column    |              Type              | Collation | Nullable |                Default
-------------+--------------------------------+-----------+----------+---------------------------------------
 id          | integer                        |           | not null | nextval('user_info_id_seq'::regclass)
 user_name   | character varying(64)          |           | not null |
 create_time | timestamp(0) without time zone |           |          | CURRENT_TIMESTAMP
Indexes:
    "user_info_pkey" PRIMARY KEY, btree (id)
``` 

- relationships （用户关联关系表)
```$xslt
                                           Table "public.relationships"
      Column      |              Type              | Collation | Nullable |                 Default
------------------+--------------------------------+-----------+----------+------------------------------------------
 id               | bigint                         |           | not null | nextval('relationship_id_seq'::regclass)
 user_id          | bigint                         |           | not null |
 other_user_id    | bigint                         |           | not null |
 user_state       | integer                        |           |          | 0
 other_user_state | integer                        |           |          | 0
 create_time      | timestamp(0) without time zone |           |          | CURRENT_TIMESTAMP
Indexes:
    "relationship_pkey" PRIMARY KEY, btree (id)
    "uniq_related_users" UNIQUE, btree (user_id, other_user_id)
```    
## 执行请求
- List all users (GET /users)
```$xslt
curl -XGET "http://localhost:9093/users"
```
- Create a user (POST /users)
```$xslt
$curl -XPOST -d '{"name":"Alice"}' "http://localhost:9093/users"
```
- List a users all relationships  (GET /users/:user_id/relationships)
```$xslt
$curl -XGET "http://localhost:9093/users/11231244213/relationships"
```
- Create/update relationship state to another user (PUT /users/:user_id/relationships/:other_user_id)
```$xslt
curl -XPUT -d '{"state":"liked"}'
"http://localhost:9093/users/11231244213/relationships/21341231231" 
```