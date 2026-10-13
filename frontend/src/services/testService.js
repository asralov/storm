export async function createTest(config) {
    const response = await fetch("/api/tests", {
        method: "POST",
        headers: {
            "Content-Type": "application/json"
        },
        body: JSON.stringify(config)
    })
    const data = await response.json()
    if (!response.ok) {
        throw new Error(data.message || "Failed to create test")
    }
    return data
}