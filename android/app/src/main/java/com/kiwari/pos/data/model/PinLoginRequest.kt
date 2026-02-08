package com.kiwari.pos.data.model

import com.google.gson.annotations.SerializedName

data class PinLoginRequest(
    @SerializedName("outlet_id")
    val outletId: String,

    @SerializedName("pin")
    val pin: String
)
