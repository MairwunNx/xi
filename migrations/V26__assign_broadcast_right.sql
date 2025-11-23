UPDATE xi_users 
SET rights = array_append(rights, 'broadcast'::user_right)
WHERE username = 'mairwunnx' 
  AND NOT ('broadcast'::user_right = ANY(rights));