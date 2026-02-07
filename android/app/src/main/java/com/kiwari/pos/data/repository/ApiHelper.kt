package com.kiwari.pos.data.repository

import com.google.gson.Gson
import com.kiwari.pos.data.model.ErrorResponse
import com.kiwari.pos.data.model.Result
import retrofit2.Response
import java.io.IOException

suspend fun <T> safeApiCall(gson: Gson, call: suspend () -> Response<T>): Result<T> {
    return try {
        val response = call()
        if (response.isSuccessful) {
            val body = response.body()
            if (body != null) Result.Success(body)
            else Result.Error("Empty response from server")
        } else {
            Result.Error(parseErrorResponse(gson, response.errorBody()?.string()))
        }
    } catch (e: IOException) {
        Result.Error("Network connection failed. Please check your internet connection.")
    } catch (e: Exception) {
        Result.Error("An unexpected error occurred: ${e.message}")
    }
}

fun parseErrorResponse(gson: Gson, errorBody: String?): String {
    return try {
        if (errorBody != null) {
            val errorResponse = gson.fromJson(errorBody, ErrorResponse::class.java)
            errorResponse.error
        } else {
            "An error occurred"
        }
    } catch (e: Exception) {
        "An error occurred"
    }
}
