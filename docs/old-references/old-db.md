// POS System Database Schema
// Enums
Enum user_role {
  MANAGER
  CASHIER
  KITCHEN
  WAITER
}
Enum order_type {
  DINE_IN
  TAKEAWAY
  DELIVERY
}
Enum order_status {
  NEW
  COOKING
  READY
  COMPLETED
  CANCELLED
}
Enum order_item_status {
  PENDING
  COOKING
  READY
}
Enum kitchen_station {
  GRILL
  BEVERAGE
  RICE
  DESSERT
}
Enum payment_method {
  CASH
  QRIS
  E_WALLET
  DEBIT
  CREDIT
}
Enum payment_status {
  PENDING
  COMPLETED
  FAILED
}
Enum stock_movement_type {
  IN
  OUT
  ADJUSTMENT
  WASTE
}
Enum promotion_type {
  PERCENTAGE
  FIXED_AMOUNT
}
// Tables
Table users {
  id uuid [pk]
  email varchar(255) [unique, not null]
  hashed_password varchar(255) [not null]
  full_name varchar(255) [not null]
  role user_role [not null]
  pin varchar(4)
  is_active boolean [default: true]
  created_at timestamp [default: `now()`]
  updated_at timestamp [default: `now()`]
  
  indexes {
    email
    role
  }
}

Table categories {
  id uuid [pk]
  name varchar(100) [not null]
  description text
  display_order int [default: 0]
  is_active boolean [default: true]
  created_at timestamp [default: `now()`]
}

Table menu_items {
  id uuid [pk]
  category_id uuid [ref: > categories.id, not null]
  name varchar(255) [not null]
  description text
  base_price decimal(10,2) [not null]
  image_url varchar(500)
  is_available boolean [default: true]
  preparation_time int
  station kitchen_station
  created_at timestamp [default: `now()`]
  updated_at timestamp [default: `now()`]
  
  indexes {
    category_id
    is_available
  }
}

Table menu_addons {
  id uuid [pk]
  menu_id uuid [ref: > menu_items.id, not null]
  addon_id uuid [ref: > addons.id, not null]
  
  indexes {
    (menu_id, addon_id) [unique]
  }
}

Table addons {
  id uuid [pk]
  name varchar(100) [not null]
  price decimal(10,2) [not null]
  is_available boolean [default: true]
  created_at timestamp [default: `now()`]
}

Table item_variants {
  id uuid [pk]
  menu_item_id uuid [ref: > menu_items.id, not null]
  name varchar(100) [not null]
  price_adjustment decimal(10,2) [default: 0]
  is_available boolean [default: true]
}

Table orders {
  id uuid [pk]
  order_number varchar(20) [unique, not null]
  table_number varchar(20)
  order_type order_type [not null]
  status order_status [not null]
  subtotal decimal(10,2) [not null]
  tax_amount decimal(10,2) [default: 0]
  service_charge decimal(10,2) [default: 0]
  discount_amount decimal(10,2) [default: 0]
  total_amount decimal(10,2) [not null]
  customer_id uuid [ref: > customers.id]
  customer_name varchar(255)
  customer_phone varchar(20)
  notes text
  created_by uuid [ref: > users.id, not null]
  created_at timestamp [default: `now()`]
  updated_at timestamp [default: `now()`]
  completed_at timestamp
  
  indexes {
    status
    created_at
    order_number
  }
}
Table order_items {
  id uuid [pk]
  order_id uuid [ref: > orders.id, not null]
  menu_item_id uuid [ref: > menu_items.id, not null]
  variant_id uuid [ref: > item_variants.id]
  quantity int [not null]
  unit_price decimal(10,2) [not null]
  subtotal decimal(10,2) [not null]
  notes text
  status order_item_status [not null]
  station kitchen_station
  created_at timestamp [default: `now()`]
  
  indexes {
    order_id
    status
  }
}

Table order_item_addons {
  id uuid [pk]
  order_item_id uuid [ref: > order_items.id, not null]
  addon_id uuid [ref: > addons.id, not null]
  quantity int [default: 1]
  unit_price decimal(10,2) [not null]
  
  indexes {
    order_item_id
  }
}
Table payments {
  id uuid [pk]
  order_id uuid [ref: > orders.id, not null]
  payment_method payment_method [not null]
  amount decimal(10,2) [not null]
  amount_received decimal(10,2)
  change_amount decimal(10,2)
  reference_number varchar(100)
  status payment_status [not null]
  processed_by uuid [ref: > users.id, not null]
  processed_at timestamp [default: `now()`]
  
  indexes {
    order_id
    payment_method
  }
}

Table promotions {
  id uuid [pk]
  name varchar(255) [not null]
  code varchar(50) [unique]
  type promotion_type [not null]
  value decimal(10,2) [not null]
  min_purchase decimal(10,2)
  max_discount decimal(10,2)
  start_date timestamp [not null]
  end_date timestamp [not null]
  is_active boolean [default: true]
  usage_limit int
  usage_count int [default: 0]
  created_at timestamp [default: `now()`]
  
  indexes {
    code
    is_active
  }
}
Table promotion_usages {
  id uuid [pk]
  promotion_id uuid [ref: > promotions.id, not null]
  order_id uuid [ref: > orders.id, not null]
  discount_amount decimal(10,2) [not null]
  used_at timestamp [default: `now()`]
}
Table attendance {
  id uuid [pk]
  user_id uuid [ref: > users.id, not null]
  clock_in timestamp [not null]
  clock_out timestamp
  total_hours decimal(5,2)
  notes text
  
  indexes {
    user_id
    clock_in
  }
}

Table customers {
  id uuid [pk]
  phone varchar(20) [unique, not null]
  name varchar(255)
  email varchar(255)
  created_at timestamp [default: `now()`]
  updated_at timestamp [default: `now()`]
  loyalty_points integer [default: 0]
  
  indexes {
    phone
  }
}